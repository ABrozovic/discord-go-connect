package discord

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type messageWriter struct {
	b            *Bot
	writeMu      sync.Mutex
	WriteBuffer  []*discordgo.MessageCreate
	writeTimer   *time.Timer
	writeCounter int
}

func newMessageWriter(b *Bot) *messageWriter {
	return &messageWriter{
		b:            b,
		WriteBuffer:  make([]*discordgo.MessageCreate, 0),
		writeTimer:   time.NewTimer(b.writeInterval),
		writeCounter: 0,
	}
}

func (mw *messageWriter) start() {
	go mw.periodicWriteToDatabase()
}

func (mw *messageWriter) AddMessage(msg *discordgo.MessageCreate) {
	mw.writeMu.Lock()
	defer mw.writeMu.Unlock()

	mw.WriteBuffer = append(mw.WriteBuffer, msg)
	mw.writeCounter++

	if len(mw.WriteBuffer) >= mw.b.maxBufferCount || mw.writeCounter >= mw.b.maxBufferCount {
		mw.writeToDatabase()
		return
	}

	mw.writeTimer.Reset(mw.b.writeInterval)
}

func (mw *messageWriter) periodicWriteToDatabase() {
	defer func () {
		if r := recover(); r!=nil {
			mw.b.logger.Debug("Recovered in %v", r)
		}
	}()
	
	for range mw.writeTimer.C {
		mw.writeToDatabase()
	}
}

func (mw *messageWriter) writeToDatabase() {
	mw.writeMu.Lock()
	defer mw.writeMu.Unlock()

	if len(mw.WriteBuffer) == 0 {
		return
	}

	err := mw.b.CreateMessage(mw.WriteBuffer)
	if err != nil {
		mw.b.logger.Error("failed to write messages to the database: %v", err)
	}

	mw.WriteBuffer = make([]*discordgo.MessageCreate, 0)
	mw.writeCounter = 0
}
