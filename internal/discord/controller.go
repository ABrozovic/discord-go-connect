package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

const (
	insertGuilds = `
			INSERT INTO Guild (id, name, icon, region, owner_id)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			icon = VALUES(icon),
			region = VALUES(region),
			owner_id = VALUES(owner_id)
		`
	insertChannels = `
			INSERT INTO Channel (id, guild_id, name, nsfw, position)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			guild_id = VALUES(guild_id),
			name = VALUES(name),
			nsfw = VALUES(nsfw),
			position = VALUES(position)
		`
	insertAuthor = `
			INSERT IGNORE INTO Author (id, email, username, avatar, bot, system)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	insertMember = `
			INSERT IGNORE INTO Member (id, guild_id, author_id, nick, avatar)
			VALUES (?, ?, ?, ?, ?)
		`
	insertMessage = `
			INSERT INTO Message (id, channel_id, guild_id, author_id, member_id, pinned, type, content)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
	// insertMessage = `
	// 	INSERT INTO Message (id, channel_id, guild_id, pinned, type, attachments, embeds, mentions, author_id)
	// 	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	// `
)

func (b *Bot) CreateOrUpdateGuilds() error {
	stmt, err := b.db.Prepare(insertGuilds)
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement: %v", err)
	}
	defer stmt.Close()

	for _, guild := range b.guilds {
		_, err := stmt.Exec(guild.ID, guild.Name, guild.Icon, guild.Region, guild.OwnerID)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement: %w", err)
		}
	}

	return nil
}

func (b *Bot) CreateOrUpdateChannels() error {
	stmt, err := b.db.Prepare(insertChannels)
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement: %w", err)
	}
	defer stmt.Close()

	for _, guild := range b.guilds {
		for _, channel := range guild.Channels {
			for _, message := range channel.Messages {
				_, err := stmt.Exec(message.ID, channel.ID, guild.ID, message.Pinned, message.Type, message.Attachments, message.Embeds, message.Mentions)
				if err != nil {
					return fmt.Errorf("failed to execute SQL statement: %w", err)
				}
			}
		}
	}

	return nil
}

func (b *Bot) CreateOrUpdateGuildsAndChannels() error {

	guildStmt, err := b.db.Prepare(insertGuilds)
	if err != nil {
		return fmt.Errorf("failed to prepare Guild SQL statement: %w", err)
	}
	defer guildStmt.Close()

	channelStmt, err := b.db.Prepare(insertChannels)
	if err != nil {
		return fmt.Errorf("failed to prepare Channel SQL statement: %w", err)
	}
	defer channelStmt.Close()

	for _, guild := range b.guilds {
		_, err := guildStmt.Exec(guild.ID, guild.Name, guild.Icon, guild.Region, guild.OwnerID)
		if err != nil {
			return fmt.Errorf("failed to execute Guild SQL statement: %w", err)
		}

		for _, channel := range guild.Channels {

			_, err := channelStmt.Exec(channel.ID, guild.ID, channel.Name, channel.NSFW, channel.Position)
			if err != nil {
				return fmt.Errorf("failed to execute Channel SQL statement: %w", err)

			}
		}
	}

	return nil
}

func (b *Bot) CreateMessage(messages []*discordgo.MessageCreate) error {
	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start database transaction: %w", err)
	}
	defer tx.Rollback()

	stmtAuthor, err := tx.Prepare(insertAuthor)
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement for authors: %w", err)
	}
	defer stmtAuthor.Close()

	stmtMember, err := tx.Prepare(insertMember)
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement for members: %w", err)
	}
	defer stmtAuthor.Close()

	stmtMessage, err := tx.Prepare(insertMessage)
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement for messages: %w", err)
	}
	defer stmtMessage.Close()

	for _, message := range messages {
		_, err := stmtAuthor.Exec(
			message.Author.ID, message.Author.Email, message.Author.Username,
			message.Author.Avatar, message.Author.Bot, message.Author.System,
		)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement for authors: %w", err)
		}

		_, err = stmtMember.Exec(
			message.Author.ID, message.Author.ID, message.GuildID, message.Member.Nick,
			message.Member.Avatar,
		)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement for members: %w", err)
		}

		_, err = stmtMessage.Exec(
			message.ID, message.ChannelID, message.GuildID, message.Author.ID, 
			message.Author.ID, message.Pinned, message.Type,
			//TODO: message.Attachments, message.Embeds, message.Mentions,
			message.Content,
		)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement for messages: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}

	return nil
}
