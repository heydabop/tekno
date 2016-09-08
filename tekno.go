package main

import (
	//"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"github.com/grafov/m3u8"
	"log"
	"net/http"
	//"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

var (
	//twitchBuffer bytes.Buffer
	discordChan chan []int16
)

func startStream() {
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
		/*mediaPlaylistRes, err := http.Get(variant.URI)
		if err != nil {
			log.Println(err)
			continue
		}
		if mediaPlaylistRes.StatusCode != 200 {
			log.Println(playlistRes.Status)
			mediaPlaylistRes.Body.Close()
			continue
		}
		p, listType, err := m3u8.DecodeFrom(mediaPlaylistRes.Body, true)
		if err != nil {
			log.Println(err)
			mediaPlaylistRes.Body.Close()
			continue
		}
		if listType != m3u8.MEDIA {
			log.Println("Expected media got master")
			mediaPlaylistRes.Body.Close()
			continue
		}
		mediaPlaylistRes.Body.Close()
		mediaPlaylist := p.(*m3u8.MediaPlaylist)
		fmt.Println(variant.URI)
		fmt.Println(mediaPlaylist)*/
		if strings.Contains(variant.URI, "audio_only") {
			audioURL = variant.URI
			break
		}
	}
	//vlc := exec.Command("cvlc", audioURL, "--sout", "#duplicate{dst=std{access=file,mux=raw,dst=-}}")
	vlc := exec.Command("cvlc", audioURL, "--sout", "#transcode{acodec=s16l,samplerate=48000,channels=2}:duplicate{dst=std{access=file,mux=raw,dst=-}}")
	vlcPipe, err := vlc.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		for {
			/*buf := make([]int16, 1920)
			_, err := vlcPipe.Read(buf)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(string(buf))
			discordChan <- buf*/
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

func main() {
	discordChan = make(chan []int16, 2)
	//twitchBuffer = bytes.NewBuffer(make([]byte, 0, 1024*1024))
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

	/*self, err := client.User("@me")
	if err == nil {
		client.UserUpdate("", "", "T̴̢̕͞E͡͏̀K̸͜Ņ́̀͘O͟͞", self.Avatar, "")
	}*/

	go startStream()

	currentVoiceSession, err = client.ChannelVoiceJoin(myGuildID, myVoiceChanID, false, false)
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	for true {
		if currentVoiceSession.Ready == false || currentVoiceSession.OpusSend == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		//dgvoice.PlayAudioFile(currentVoiceSession, "Sandstorm.mp3")
		dgvoice.SendPCM(currentVoiceSession, discordChan)
		time.Sleep(1 * time.Second)
		dgvoice.KillPlayer()
		time.Sleep(5 * time.Second)
	}
}
