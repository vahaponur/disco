package main

import (
	"bufio"
	"github.com/bwmarrin/discordgo"
	"github.com/hraban/opus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	PCMFrameSize = 960
	Channels     = 2
	PCMChunkSize = PCMFrameSize * Channels * 2 // 2 for 16-bit depth
	SampleRate   = 48000
)

var (
	downloadPath string
)

type Player struct {
	Next            bool
	Pause           bool
	Playing         bool
	Downloading     bool
	NewQueueStarted bool
	Queue           chan string

	GuildID      string
	ChannelID    string
	Session      *discordgo.Session
	VoiceChannel *discordgo.VoiceConnection
	Processes    sync.WaitGroup
}

func NewPlayer(session *discordgo.Session) *Player {
	return &Player{Session: session}
}

func main() {
	NewConfig(os.Getenv("CONFIG_PATH"), os.Getenv("CONFIG_NAME"))

	dir, err := os.MkdirTemp("", "disco_")
	if err != nil {
		logrus.Fatal("Failed to while creating to temp folder", err)
		return
	}
	defer os.RemoveAll(dir)
	logrus.Info("Temp directory: " + dir)
	downloadPath = dir

	session, err := discordgo.New("Bot " + viper.GetString("TOKEN"))
	if err != nil {
		logrus.Fatal("Error creating Discord session: ", err)
	}

	player := NewPlayer(session)
	session.AddHandler(player.CommandHandler)

	if err = session.Open(); err != nil {
		logrus.Fatal("Error opening Discord session: ", err)
	}
	defer session.Close()
	logrus.Info("Session started")

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	<-shutdownCh
	logrus.Info("Received shutdown signal, exiting...")
}

func (p *Player) CommandHandler(session *discordgo.Session, m *discordgo.MessageCreate) {

	message := m.Content
	if m.Author.ID == session.State.User.ID {
		return
	}

	if message == "pause" {
		p.Pause = true
		return
	}

	if message == "play" {
		p.Pause = false
		return
	}

	if message == "next" {
		p.Next = true
		return
	}

	if !strings.HasPrefix(m.Content, "spotify ") {
		return
	}

	url := strings.ReplaceAll(strings.TrimPrefix(m.Content, "spotify "), " ", "")
	if url == "" {
		return
	}

	if p.Playing {
		p.NewQueueStarted = true
		logrus.Info("Cleaning up the queue...")
		p.Processes.Wait()

		p.NewQueueStarted = false
		logrus.Info("New queue started")
	}

	voiceChannelID := FindUserVoiceChannel(p.Session, m.GuildID, m.Author.ID)
	vc, err := p.Session.ChannelVoiceJoin(m.GuildID, voiceChannelID, false, true)
	if err != nil {
		logrus.Error("Error joining voice channel: ", err)
		return
	}

	p.GuildID = m.GuildID
	p.ChannelID = m.ChannelID
	p.VoiceChannel = vc
	p.Queue = make(chan string)

	go func() {
		_, err = p.Session.ChannelMessageSend(p.ChannelID, "Downloading, please wait...")
		if err != nil {
			logrus.Error("ChannelMessageSend error: ", err)
		}
		p.Download(url)
	}()

	p.WaitForQueue()
}

func (p *Player) WaitForQueue() {
	p.Processes.Add(1)
	defer func() {
		logrus.Info("WaitForQueue [DONE]")
		p.Processes.Done()
	}()

	for file := range p.Queue {
		if p.NewQueueStarted {
			return
		}

		err := ConvertMP3ToPCM(downloadPath + "/" + file)
		if err != nil {
			logrus.Error("ConvertMP3ToPCM error: ", err)
		}
		p.Session.ChannelMessageSend(p.ChannelID, "Playing: "+file)
		p.Play(p.VoiceChannel)
	}
}

func (p *Player) Play(vc *discordgo.VoiceConnection) {
	p.Processes.Add(1)
	defer func() {
		logrus.Info("Play [DONE]")
		p.Playing = false
		p.Processes.Done()
	}()

	p.Pause = false
	p.Playing = true

	logrus.Info("Reading PCM... ")
	pcmData, err := os.ReadFile("temp.pcm")
	if err != nil {
		logrus.Error("Error reading PCM file: ", err)
		return
	}

	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppVoIP)
	if err != nil {
		logrus.Error("Error creating Opus encoder: ", err)
		return
	}

	for i := 0; i < len(pcmData); i += PCMChunkSize {

		if p.NewQueueStarted {
			return
		}

		if p.Next {
			p.Next = false
			return
		}

		for p.Pause {
			time.Sleep(time.Second * 2)
		}

		if i+PCMChunkSize > len(pcmData) {
			return
		}

		pcmChunk := pcmData[i : i+PCMChunkSize]
		int16s := BytesToInt16s(pcmChunk)
		opusFrame := make([]byte, PCMChunkSize)

		n, err := encoder.Encode(int16s, opusFrame)
		if err != nil {
			logrus.Error("Error encoding PCM to Opus: ", err)
			return
		}

		vc.OpusSend <- opusFrame[:n]
	}
}

func (p *Player) Download(url string) {
	_ = os.RemoveAll(downloadPath)
	err := os.Mkdir(downloadPath, os.ModePerm)
	if err != nil {
		logrus.Error("Failed while creating folder to download: ", err)
		return
	}

	p.Processes.Add(1)
	defer func() {
		logrus.Info("Download [DONE]")
		close(p.Queue)
		p.Downloading = false
		p.Processes.Done()
	}()

	downloadedFiles := make(map[string]bool)
	p.Downloading = true

	logrus.Info("Playlist downloading...")
	cmd := exec.Command("spotdl", url)
	cmd.Dir = downloadPath
	stdout, _ := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		logrus.Error("Cmd start error: ", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		if p.NewQueueStarted {
			cmd.Process.Kill()
			return
		}

		res := scanner.Text()
		logrus.Info("spotdl: ", res)

		if strings.Contains(res, "Downloaded") {
			files, err := os.ReadDir(downloadPath)
			if err != nil {
				logrus.Error("Readdir error ", err)
				return
			}

			for _, file := range files {
				_, exists := downloadedFiles[file.Name()]
				if !exists && p.Downloading {
					p.Queue <- file.Name()
					downloadedFiles[file.Name()] = true
				}
			}
		}

	}
	err = cmd.Wait()
	if err != nil {
		logrus.Error("Cmd wait error: ", err)
	}
}
