package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

var token string
var buffer = make([][]byte, 0)
var run *exec.Cmd
// パッケージの初期化
// Bot のTokenを設定する
func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}
func main() {
	if token == "" {
		fmt.Println("No token provided. Please run: tomato.go -t <bot token>")
		return
	}

	// 音声ファイルを読み込む
	err := loadSound()
	if err != nil {
		fmt.Println("Error loading sound file:", err)
		return
	}

	//discordの初期設定
	discord, err := discordgo.New("Bot "+token)
	if err != nil {
		fmt.Println("Error creating Discord session", err)
		return
	}
	discord.AddHandler(ready)
	discord.AddHandler(messageCreate)
	discord.AddHandler(guildCreate)

	// ギルドに関する情報を取得
	discord.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	// websocketで通信を開始する
	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord session:", err)
	}

	// コントロールcを押されるまでまつ
	fmt.Println("tomato bot running Please CTRL-C to exit")
	sc := make(chan os.Signal, 1) // シグナルを受け取るチャンネルをつくる
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill) // 受け取るシグナルを設定
	<- sc // シグナルを受け取るまで終了しないようにする

	//discordのセッションを終了する
	discord.Close()
}

// この関数は、ボットがDiscordから "ready "イベントを受信したときに、（上記のAddHandlerにより）呼び出される
func ready(s *discordgo.Session, event *discordgo.Ready) {
	
	s.UpdateGameStatus(0, "!start")
}

// ギルドの作成をしてチャンネルにメッセージを送信する
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			_, _ = s.ChannelMessageSend(channel.ID, "totatoBotの準備が出来ました。!startコマンドで音楽を再生します")
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// bot自身の発言を無視する
	if m.Author.ID == s.State.User.ID {
		return
	}
	// startコマンドを誰かが発言したとき
	if strings.HasPrefix(m.Content, "!start") {
		
		//コマンドが書かれたチャンネルをみつける
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			fmt.Println("not find channel")
			return
		}
		// ギルドを探す
		g, err := s.State.Guild(c.GuildID)
		if err != nil {
			fmt.Println("not find guild")
			return
		}

		for _, vs := range g.VoiceStates {
			if vs.UserID == m.Author.ID {
				err = playSound(s, g.ID, vs.ChannelID)
				if err != nil {
					fmt.Println("Error playing sound:", err)
				}
				return
			}
		}
	}

}

// mp3ファイルの読み込み
func loadSound() error {
	fileName := "../A_Peaceful_Christmas.mp3"
	opts := dca.StdEncodeOptions
	opts.RawOutput = true
	opts.Bitrate = 120
	encodeSession, err := dca.EncodeFile(fileName, opts)
	if err != nil {
		return err
	}
	// dca file を作成
	output, err := os.Create("bgm.dca")
	if err != nil {
		fmt.Println("Error Create File:", err)
		return err
	}
	io.Copy(output, encodeSession)
	defer encodeSession.Cleanup()
	file, err := os.Open("bgm.dca")
	if err != nil {
		fmt.Println("Error open dca file:", err)
		return err
	}

	var opuslen int16
	for {
		// encodeしたファイルを読み込む
		err = binary.Read(file, binary.LittleEndian, &opuslen)
		// 正常に終了もしくは強制終了したときはfileを閉じる
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			fmt.Println("Error encode dca:", err)
			return err
		}
		// encodeした情報を読み込む
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)
		if err != nil {
			fmt.Println("Error reading from dca file", err)
			return err
		}
		// encodeしたデータを追加する
		buffer = append(buffer, InBuf)
	}
}

func playSound(s *discordgo.Session, gulidID, channelID string) (err error) {

	vc, err := s.ChannelVoiceJoin(gulidID, channelID, false, true)
	if err != nil {
		return 
	}

	// 音楽再生まで少し時間を空ける
	time.Sleep(250*time.Millisecond)
	vc.Speaking(true)

	// bufferデータを送信する
	for _, buff := range buffer {
		vc.OpusSend <- buff
	}
	
	// 話すのをやめる
	vc.Speaking(false)
	time.Sleep(250 * time.Millisecond)
	// ボイスチャンネルから切断する
	vc.Disconnect()
	return nil
}