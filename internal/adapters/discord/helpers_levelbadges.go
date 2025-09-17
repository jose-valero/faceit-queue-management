package discord

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// --- 1) Mapa por defecto con TUS IDs (se usa si no hay ENV) ---
var defaultLevelEmojiMap = map[int]string{
	1:  "<:faceitlvl1:1414748660872384633>",
	2:  "<:faceitlvl2:1414748674361262171>",
	3:  "<:faceitlvl3:1414748685463457812>",
	4:  "<:faceitlvl4:1414748695898886175>",
	5:  "<:faceitlvl5:1414748709303877632>",
	6:  "<:faceitlvl6:1414748729545723986>",
	7:  "<:faceitlvl7:1414748743764414525>",
	8:  "<:faceitlvl8:1414748755261132960>",
	9:  "<:faceitlvl9:1414749476941336666>",
	10: "<:faceitlvl10:1414748834889859113>",
}

// --- 2) ENV override opcional ---
// Formato: FACEIT_LEVEL_EMOJIS="1:<:name:id>,2:<:name:id>,...,10:<:name:id>"
var emojiMarkupRe = regexp.MustCompile(`^<:[a-zA-Z0-9_~]+:\d+>$`)

func parseEmojiMapEnv(s string) map[int]string {
	out := map[int]string{}
	if strings.TrimSpace(s) == "" {
		return out
	}
	pairs := strings.Split(s, ",")
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		colon := strings.IndexByte(p, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(p[:colon])
		val := strings.TrimSpace(p[colon+1:])
		n, err := strconv.Atoi(key)
		if err != nil || n < 1 || n > 10 {
			continue
		}
		if !emojiMarkupRe.MatchString(val) {
			// aceptamos igual, pero ideal que sea "<:name:id>"
			// continue
		}
		out[n] = val
	}
	return out
}

// --- 3) Descubrimiento automático por nombre (fallback) ---
var emojiLevelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(?:faceit|faceitlvl|lvl|level|lv|l)?_?0*([1-9]|10)$`),
	regexp.MustCompile(`(?i)^(?:faceit_)?level_?0*([1-9]|10)$`),
	regexp.MustCompile(`(?i)^(?:faceit)?0*([1-9]|10)$`),
}

func parseLevelFromEmojiName(name string) (int, bool) {
	name = strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	for _, re := range emojiLevelPatterns {
		if m := re.FindStringSubmatch(name); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

// --- 4) Inicialización del mapa en el Router ---
func (r *Router) initLevelBadges() {
	r.levelEmojis = make(map[int]string, 12)

	// (a) ENV override
	if v := os.Getenv("FACEIT_LEVEL_EMOJIS"); v != "" {
		if m := parseEmojiMapEnv(v); len(m) > 0 {
			for k, val := range m {
				r.levelEmojis[k] = val
			}
			return
		}
	}

	// (b) Default estático (tus IDs)
	for k, val := range defaultLevelEmojiMap {
		r.levelEmojis[k] = val
	}

	// (c) Intento de autodescubrimiento en el guild: si encuentra, pisa el default
	g, _ := r.s.State.Guild(r.guildID)
	if g == nil {
		g, _ = r.s.Guild(r.guildID)
		if g != nil {
			_ = r.s.State.GuildAdd(g)
		}
	}
	if g == nil {
		return
	}
	for _, e := range g.Emojis {
		if lvl, ok := parseLevelFromEmojiName(e.Name); ok {
			r.levelEmojis[lvl] = fmt.Sprintf("<:%s:%s>", e.Name, e.ID)
		}
	}
}

// --- 5) API de uso en el render ---
func (r *Router) levelBadge(level int) string {
	if level < 1 || level > 10 {
		return ""
	}
	if r.levelEmojis != nil {
		if v := r.levelEmojis[level]; v != "" {
			return v
		}
	}
	// fallback Unicode por si en otro guild no se ven los custom
	switch level {
	case 10:
		return "🔟"
	case 9:
		return "9️⃣"
	case 8:
		return "8️⃣"
	case 7:
		return "7️⃣"
	case 6:
		return "6️⃣"
	case 5:
		return "5️⃣"
	case 4:
		return "4️⃣"
	case 3:
		return "3️⃣"
	case 2:
		return "2️⃣"
	case 1:
		return "1️⃣"
	default:
		return ""
	}
}
