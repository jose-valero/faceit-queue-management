package discord

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
)

var reMention = regexp.MustCompile(`<@!?(\d+)>`)

func parseIDs(raw string) []string {
	ids := []string{}
	for _, tok := range strings.Fields(raw) {
		if m := reMention.FindStringSubmatch(tok); len(m) == 2 {
			ids = append(ids, m[1])
			continue
		}
		allDigits := true
		for _, r := range tok {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			ids = append(ids, tok)
		}
	}
	return ids
}

func faceitPlayerURL(nick string) string {
	return "https://www.faceit.com/en/players/" + url.PathEscape(nick)
}

func fmtRemain(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func optStr(ic *discordgo.InteractionCreate, name string) (string, bool) {
	if ic.Type != discordgo.InteractionApplicationCommand {
		return "", false
	}
	for _, o := range ic.ApplicationCommandData().Options {
		if o.Name == name {
			return o.StringValue(), true
		}
		// subcommand
		if o.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, so := range o.Options {
				if so.Name == name {
					return so.StringValue(), true
				}
			}
		}
	}
	return "", false
}

func optBool(ic *discordgo.InteractionCreate, name string) (bool, bool) {
	if ic.Type != discordgo.InteractionApplicationCommand {
		return false, false
	}
	for _, o := range ic.ApplicationCommandData().Options {
		if o.Name == name && o.Type == discordgo.ApplicationCommandOptionBoolean {
			return o.BoolValue(), true
		}
		if o.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, so := range o.Options {
				if so.Name == name && so.Type == discordgo.ApplicationCommandOptionBoolean {
					return so.BoolValue(), true
				}
			}
		}
	}
	return false, false
}

func optInt(ic *discordgo.InteractionCreate, name string) (int, bool) {
	if ic.Type != discordgo.InteractionApplicationCommand {
		return 0, false
	}
	for _, o := range ic.ApplicationCommandData().Options {
		if o.Name == name && o.Type == discordgo.ApplicationCommandOptionInteger {
			return int(o.IntValue()), true
		}
		if o.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, so := range o.Options {
				if so.Name == name && so.Type == discordgo.ApplicationCommandOptionInteger {
					return int(so.IntValue()), true
				}
			}
		}
	}
	return 0, false
}

func subcmdName(ic *discordgo.InteractionCreate) (string, bool) {
	if ic.Type != discordgo.InteractionApplicationCommand {
		return "", false
	}
	for _, o := range ic.ApplicationCommandData().Options {
		if o.Type == discordgo.ApplicationCommandOptionSubCommand {
			return o.Name, true
		}
	}
	return "", false
}

func (r *Router) statusSuffix(it service.QueueItemRich, graceAFK, graceLeft time.Duration) (string, time.Duration) {
	var nextRefresh time.Duration
	suf := " (waiting)"

	switch it.Status {
	case "left":
		if graceLeft > 0 {
			until := it.LastSeenAt.Add(graceLeft)
			remain := time.Until(until)
			if remain <= 5*time.Second {
				suf = " (left " + fmtRemain(remain) + ")"
			} else {
				suf = fmt.Sprintf(" (left <t:%d:R>)", until.Unix())
			}
			if remain > 0 && (nextRefresh == 0 || remain < nextRefresh) {
				nextRefresh = remain
			}
		} else {
			suf = " (left)"
		}
	case "afk":
		if graceAFK > 0 {
			until := it.LastSeenAt.Add(graceAFK)
			remain := time.Until(until)
			if remain <= 5*time.Second {
				suf = " (afk " + fmtRemain(remain) + ")"
			} else {
				suf = fmt.Sprintf(" (afk <t:%d:R>)", until.Unix())
			}
			if remain > 0 && (nextRefresh == 0 || remain < nextRefresh) {
				nextRefresh = remain
			}
		} else {
			suf = " (afk)"
		}
	}
	return suf, nextRefresh
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
