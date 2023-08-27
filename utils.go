package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

func BytesToInt16s(data []byte) []int16 {
	int16s := make([]int16, len(data)/2)
	for i := range int16s {
		int16s[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}
	return int16s
}

func ConvertMP3ToPCM(filePath string) error {
	os.Remove("temp.pcm")
	logrus.Info("File converting to PCM...")
	cmd := exec.Command("ffmpeg", "-i", filePath, "-f", "s16le", "-ar", "48000", "-ac", "2", "temp.pcm", "-y")
	return cmd.Run()
}

func FindUserVoiceChannel(session *discordgo.Session, guildID, userID string) string {
	guild, _ := session.State.Guild(guildID)
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID
		}
	}
	return ""
}
