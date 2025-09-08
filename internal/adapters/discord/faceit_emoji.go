package discord

import "fmt"

var FaceitLevelEmoji = map[int]string{
	1:  "<:faceitlvl1:1414375523869790238>",
	2:  "<:faceitlvl2:1414375622670815273>",
	3:  "<:faceitlvl3:1414375683567779920>",
	4:  "<:faceitlvl4:1414376165610885291>",
	5:  "<:faceitlvl5:1414376163593294005>",
	6:  "<:faceitlvl6:1414376161714110596>",
	7:  "<:faceitlvl7:1414376160095113266>",
	8:  "<:faceitlvl8:1414376158304141353>",
	9:  "<:faceitlvl9:1414376155687157831>",
	10: "<:faceitlvl10:1414376153254461482>",
}

func levelBadge(lvl *int) string {
	if lvl == nil {
		return ""
	}
	if e, ok := FaceitLevelEmoji[*lvl]; ok {
		return e
	}
	return fmt.Sprintf("[Lv %d]", *lvl)
}
