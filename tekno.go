package main

import (
	"fmt"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	client, err := discordgo.New(botToken)
	if err != nil {
		fmt.Println(err)
		return
	}
	client.StateEnabled = true
	var currentVoiceSession *discordgo.VoiceConnection

	client.Open()
	defer client.Close()
	defer client.Logout()
	defer func() {
		if currentVoiceSession != nil {
			err := currentVoiceSession.Disconnect()
			if err != nil {
				fmt.Println("ERROR leaving voice channel " + err.Error())
			}
		}
	}()

	signals := make(chan os.Signal, 1)

	go func() {
		select {
		case <-signals:
			if currentVoiceSession != nil {
				err := currentVoiceSession.Disconnect()
				if err != nil {
					fmt.Println("ERROR leaving voice channel " + err.Error())
				}
			}
			client.Logout()
			client.Close()
			os.Exit(0)
		}
	}()
	signal.Notify(signals, os.Interrupt)

	/*self, err := client.User("@me")
	if err == nil {
		client.UserUpdate("", "", "T̴̢̕͞E͡͏̀K̸͜Ņ́̀͘O͟͞", self.Avatar, "")
	}*/

	currentVoiceSession, err = client.ChannelVoiceJoin("98470233999675392", "129972724922580992", false, false)
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	for true {
		if currentVoiceSession.Ready == false || currentVoiceSession.OpusSend == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		dgvoice.PlayAudioFile(currentVoiceSession, "Sandstorm.mp3")
		time.Sleep(1 * time.Second)
		dgvoice.KillPlayer()
		time.Sleep(5 * time.Second)
	}
}