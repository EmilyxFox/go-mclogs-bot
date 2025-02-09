package main

import (
	"fmt"
	"io"
	"log/slog"
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

	logger := slog.With(
		slog.Group("message",
			slog.String("id", m.ID),
			"attachments", m.Attachments,
		),
		slog.Group("channel",
			slog.String("id", m.ChannelID),
		),
		slog.Group("author",
			slog.String("id", m.Author.ID),
			slog.String("name", m.Author.Username),
		),
	)

	typingStarted := false
	for _, a := range m.Attachments {
		if !strings.HasPrefix(a.ContentType, "text/plain") {
			continue
		}

		if !typingStarted {
			if err := s.ChannelTyping(m.ChannelID); err != nil {
				logger.Error("Error starting typing indicator", "error", err)
			}
			typingStarted = true
		}

		logger.Info("Downloading attachment", "url", a.URL)
		resp, err := http.Get(a.URL)
		if err != nil {
			logger.Error("Failed to download file", "error", err, "url", a.URL)
			continue
		}
		defer resp.Body.Close()

		logger.Info("Reading content", "url", a.URL)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Failed to read file content", "error", err, "url", a.URL)
			continue
		}

		if len(body) > 10*1024*1024 /* 10MiB */ {
			logger.Info("File too large, skipping", "size", len(body), "url", a.URL)
			continue
		}

		pr, err := mclc.PasteLog(string(body))
		if err != nil {
			logger.Error("Failed to paste log", "error", err, "url", a.URL)
			return
		}

		logger.Info("Pasted log successfully", "response", pr)

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
			logger.Error("Error fetching bot user", "error", err)
		} else {
			re.Author.IconURL = botUser.AvatarURL("32")
		}

		_, err = s.ChannelMessageSendEmbed(m.ChannelID, re)
		if err != nil {
			logger.Error("Failed to send message to Discord", "error", err)
		}
	}
}

func main() {
	discordToken, present := os.LookupEnv("DISCORD_TOKEN")
	if !present {
		slog.Error("No discord bot token supplied")
		os.Exit(1)
	}

	discord, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		slog.Error("Error registering bot", "error", err)
		os.Exit(1)
	}

	discord.Identify.Intents += discordgo.IntentMessageContent

	discord.AddHandler(handleMessageCreate)

	err = discord.Open()
	if err != nil {
		slog.Error("Error opening Discord session", "error", err)
		os.Exit(1)
	}

	botUser, err := discord.User("@me")
	if err != nil {
		slog.Error("Error fetching bot user", "error", err)
	}
	slog.Info(fmt.Sprintf("Logged in as %v#%v", botUser.Username, botUser.Discriminator))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down...")
	discord.Close()
}
