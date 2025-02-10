package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
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
		),
		slog.Group("channel",
			slog.String("id", m.ChannelID),
		),
		slog.Group("author",
			slog.String("id", m.Author.ID),
			slog.String("name", m.Author.Username),
		),
	)

	logger.Info("Found message attachments", "attachments", m.Attachments)

	ch := make(chan *discordgo.MessageAttachment)
	typingStarted := false
	go func() {
		for _, at := range m.Attachments {
			if strings.HasPrefix(at.ContentType, "text/plain") {
				if !typingStarted {
					if err := s.ChannelTyping(m.ChannelID); err != nil {
						logger.Error("Error starting typing indicator", "error", err)
					}
					typingStarted = true
				}
				logger.Debug("Sending attachment to channel")
				ch <- at
			}
		}
		close(ch)
	}()

	var fs []*discordgo.MessageEmbedField
	var mu sync.Mutex
	var wg sync.WaitGroup

	for at := range ch {
		wg.Add(1)
		go func(at *discordgo.MessageAttachment) {
			defer wg.Done()

			logger.Info("Downloading attachment", "url", at.URL)
			resp, err := http.Get(at.URL)
			if err != nil {
				logger.Error("Failed to download file", "error", err, "url", at.URL)
				return
			}
			defer resp.Body.Close()

			logger.Info("Reading content", "url", at.URL)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				logger.Error("Failed to read file content", "error", err, "url", at.URL)
				return
			}

			if len(body) > 10*1024*1024 /* 10MiB */ {
				logger.Info("File too large, skipping", "size", len(body), "url", at.URL)
				return
			}

			pr, err := mclc.PasteLog(string(body))
			if err != nil {
				logger.Error("Failed to paste log", "error", err, "url", at.URL)
				return
			}

			logger.Info("Pasted log successfully", "response", pr)

			an, err := mclc.GetInsights(pr.ID)
			if err != nil {
				logger.Error("Failed to get paste insights", "error", err, "id", pr.ID)
			}

			logger.Info("Retrieved log insights successfully", "response", an)

			mu.Lock()
			fs = append(fs, &discordgo.MessageEmbedField{
				Name:   an.Title,
				Value:  pr.URL,
				Inline: true,
			})
			mu.Unlock()
		}(at)
	}

	wg.Wait()

	if len(fs) == 0 {
		logger.Info("No pastes uploaded.")
		return
	}

	logger.Info(fmt.Sprintf("Uploaded %v files.", len(fs)), "pastes", fs)

	re := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: "mclo.gs",
			URL:  "https://mclo.gs/",
		},
		Title:       "Your logs were uploaded for easier reading",
		Description: "-# [Why?](https://github.com/EmilyxFox/go-mclogs-bot/blob/main/why.md) [Source](https://github.com/emilyxfox/go-mclogs-bot)",
		Fields:      fs,
		Color:       0x2d3943,
		Timestamp:   time.Now().Format(time.RFC3339),
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

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

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
