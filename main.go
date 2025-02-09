package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilyxfox/go-mclogs-bot/mclogs"
)

var mclc = mclogs.NewClient()

func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(m.Attachments) < 1 || len(m.Attachments) > 5 {
		return
	}
	log.Printf("Attachments: %#+v", m.Attachments[0])

	typingStarted := false
	for _, a := range m.Attachments {
		if !strings.HasPrefix(a.ContentType, "text/plain") {
			continue
		}

		if !typingStarted {
			if err := s.ChannelTyping(m.ChannelID); err != nil {
				log.Printf("Error starting typing indicator: %v", err)
			}
			typingStarted = true
		}

		log.Print("Downloading attachment...")
		resp, err := http.Get(a.URL)
		if err != nil {
			log.Printf("Failed to download file: %v", err)
			continue
		}
		defer resp.Body.Close()

		log.Print("Reading content...")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read file content: %v", err)
			continue
		}

		if len(body) > 10*1024*1024 /* 10MiB */ {
			log.Println("File too large, skipping...")
			continue
		}

		pr, err := mclc.PasteLog(string(body))
		if err != nil {
			log.Fatalf("Failed to paste log: %v", err)
		}

		log.Printf("%+#v", pr)
		re := &discordgo.MessageEmbed{
			Description: fmt.Sprintf("Your logs were uploaded for easier reading:\n%s", pr.URL),
			Color:       0x2d3943,
			Timestamp:   time.Now().Format(time.RFC3339),
			Author: &discordgo.MessageEmbedAuthor{
				Name: "mclo.gs",
				URL:  "https://mclo.gs/",
			},
		}

		botUser, err := s.User("@me")
		if err != nil {
			fmt.Println("Error fetching bot user,", err)
		} else {
			re.Author.IconURL = botUser.AvatarURL("32")
		}

		_, err = s.ChannelMessageSendEmbed(m.ChannelID, re)
		if err != nil {
			log.Printf("Failed to send message to Discord: %v", err)
		}
	}
}

func main() {
	discordToken, present := os.LookupEnv("DISCORD_TOKEN")
	if !present {
		log.Fatal("No discord bot token supplied")
	}

	discord, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Error registering bot: %v", err)
	}

	discord.Identify.Intents += discordgo.IntentMessageContent

	discord.AddHandler(handleMessageCreate)

	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening Discord session,", err)
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	discord.Close()
}
