package main

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"github.com/grafov/m3u8"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

func startStream(discordChan chan []int16) {
	res, err := http.Get("http://api.twitch.tv/api/channels/monstercat/access_token")
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalln(res.Status)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var token struct {
		Token string `json:"token"`
		Sig   string `json:"sig"`
	}
	err = json.Unmarshal(body, &token)
	if err != nil {
		log.Fatalln(err)
	}

	playlistRes, err := http.Get(fmt.Sprintf("http://usher.twitch.tv/api/channel/hls/monstercat.m3u8?player=twitchweb&&token=%s&sig=%s&allow_audio_only=true&allow_source=true&type=any&p=2015", strings.Replace(token.Token, `\"`, `"`, 0), token.Sig))
	if err != nil {
		log.Fatalln(err)
	}
	defer playlistRes.Body.Close()
	if playlistRes.StatusCode != 200 {
		log.Fatalln(playlistRes.Status)
	}

	masterPlaylist := m3u8.NewMasterPlaylist()
	if err = masterPlaylist.DecodeFrom(playlistRes.Body, true); err != nil {
		log.Fatalln(err)
	}
	var audioURL string
	for _, variant := range masterPlaylist.Variants {
		if strings.Contains(variant.URI, "audio_only") {
			audioURL = variant.URI
			break
		}
	}

	vlc := exec.Command("cvlc", audioURL, "--sout", "#transcode{acodec=s16l,samplerate=48000,channels=2}:duplicate{dst=std{access=file,mux=raw,dst=-}}")
	vlcPipe, err := vlc.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		for {
			discordBuffer := make([]int16, 1920)
			err = binary.Read(vlcPipe, binary.LittleEndian, &discordBuffer)
			if err != nil {
				log.Fatalln(err)
			}
			discordChan <- discordBuffer
		}
	}()
	if err := vlc.Run(); err != nil {
		log.Fatalln(err)
	}
}

func updateAvatar(session *discordgo.Session, self *discordgo.User) {
	avatar, err := os.Open("avatar.png")
	if err != nil {
		return
	}
	defer avatar.Close()

	info, err := avatar.Stat()
	if err != nil {
		return
	}
	buf := make([]byte, info.Size())

	reader := bufio.NewReader(avatar)
	reader.Read(buf)

	avatarBase64 := base64.StdEncoding.EncodeToString(buf)
	avatarBase64 = fmt.Sprintf("data:image/png;base64,%s", avatarBase64)

	_, err = session.UserUpdate("", "", self.Username, avatarBase64, "")
}

func updateName(session *discordgo.Session, self *discordgo.User, newName string) {
	session.UserUpdate("", "", newName, self.Avatar, "")
}

func main() {
	discordChan := make(chan []int16, 2)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	const myGuildID = "98470233999675392"
	const myVoiceChanID = "129972724922580992"
	client, err := discordgo.New(botToken)
	if err != nil {
		fmt.Println(err)
		return
	}
	client.StateEnabled = true
	var currentVoiceSession *discordgo.VoiceConnection

	self, err := client.User("@me")
	if err != nil {
		fmt.Println(err)
		return
	}

	client.AddHandler(func(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
		if v.UserID != self.ID {
			return
		}
		if len(v.ChannelID) == 0 || v.ChannelID != myVoiceChanID {
			s.ChannelVoiceJoin(myGuildID, myVoiceChanID, false, false)
		}
	})

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

	//updateAvatar(client, self)
	//updateName(client, self, "T̴̢̕͞E͡͏̀K̸͜Ņ́̀͘O͟͞")

	go startStream(discordChan)

	currentVoiceSession, err = client.ChannelVoiceJoin(myGuildID, myVoiceChanID, false, false)
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	for currentVoiceSession.Ready == false || currentVoiceSession.OpusSend == nil {
		time.Sleep(2 * time.Second)
	}
	dgvoice.SendPCM(currentVoiceSession, discordChan)
}
