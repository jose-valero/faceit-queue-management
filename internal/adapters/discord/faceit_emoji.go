package discord

import "fmt"

var FaceitLevelEmoji = map[int]string{
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

func levelBadge(lvl *int) string {
	if lvl == nil {
		return ""
	}
	if e, ok := FaceitLevelEmoji[*lvl]; ok {
		return e
	}
	return fmt.Sprintf("[Lv %d]", *lvl)
}
