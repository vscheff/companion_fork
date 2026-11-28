package companion

import (
	"bytes"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"
)

const (
	combatTokenStart = '\x01'
	combatTokenEnd   = '\x02'
)

var combatClassIDs = map[string]int{
	"WARRIOR": 1, "PALADIN": 2, "HUNTER": 3, "ROGUE": 4, "PRIEST": 5, "DEATHKNIGHT": 6, "SHAMAN": 7,
	"MAGE": 8, "WARLOCK": 9, "MONK": 10, "DRUID": 11, "DEMONHUNTER": 12, "EVOKER": 13,
}

type combatClassSpell struct {
	id   uint64
	name string
}

var combatClassCasts = map[combatClassSpell]string{
	{78, "Heroic Strike"}:     "WARRIOR",
	{100, "Charge"}:           "WARRIOR",
	{284, "Heroic Strike"}:    "WARRIOR",
	{20271, "Judgement"}:      "PALADIN",
	{639, "Holy Light"}:       "PALADIN",
	{1130, "Hunter's Mark"}:   "HUNTER",
	{13549, "Serpent Sting"}:  "HUNTER",
	{1752, "Sinister Strike"}: "ROGUE",
	{1757, "Sinister Strike"}: "ROGUE",
	{591, "Smite"}:            "PRIEST",
	{8092, "Mind Blast"}:      "PRIEST",
	{2052, "Lesser Heal"}:     "PRIEST",
	{403, "Lightning Bolt"}:   "SHAMAN",
	{529, "Lightning Bolt"}:   "SHAMAN",
	{331, "Healing Wave"}:     "SHAMAN",
	{332, "Healing Wave"}:     "SHAMAN",
	{8050, "Flame Shock"}:     "SHAMAN",
	{143, "Fireball"}:         "MAGE",
	{2136, "Fire Blast"}:      "MAGE",
	{172, "Corruption"}:       "WARLOCK",
	{980, "Bane of Agony"}:    "WARLOCK",
	{5176, "Wrath"}:           "DRUID",
	{5178, "Wrath"}:           "DRUID",
	{8924, "Moonfire"}:        "DRUID",
}

var combatClassSpellNames = map[string][]string{
	"WARRIOR": {"Heroic Strike", "Charge", "Rend", "Thunder Clap", "Hamstring", "Shield Bash", "Revenge", "Overpower", "Sunder Armor", "Battle Shout", "Demoralizing Shout", "Execute", "Cleave", "Mortal Strike", "Bloodthirst", "Shield Slam", "Whirlwind", "Slam", "Intercept", "Pummel", "Mocking Blow", "Bloodrage", "Shield Block", "Intimidating Shout", "Piercing Howl", "Battle Stance", "Defensive Stance", "Berserker Stance", "Charge Stun", "Taunt"},
	"PALADIN": {"Judgement", "Holy Light", "Flash of Light", "Seal of Righteousness", "Seal of the Crusader", "Seal of Command", "Hammer of Justice", "Holy Strike", "Crusader Strike", "Consecration", "Exorcism", "Holy Shock", "Hammer of Wrath", "Devotion Aura", "Retribution Aura", "Blessing of Might", "Divine Protection", "Lay on Hands", "Holy Shield", "Seal of Fury", "Judgement of Righteousness", "Judgement of Fury", "Judgement of the Crusader", "Blessing of Kings", "Blessing of Wisdom", "Righteous Fury", "Summon Warhorse", "Redoubt", "Sense Undead", "Forbearance", "Concentration Aura", "Shadow Resistance Aura"},
	"HUNTER":  {"Auto Shot", "Arcane Shot", "Serpent Sting", "Hunter's Mark", "Raptor Strike", "Multi-Shot", "Aimed Shot", "Concussive Shot", "Mongoose Bite", "Wing Clip", "Steady Shot", "Immolation Trap", "Explosive Trap", "Freezing Trap", "Aspect of the Hawk", "Aspect of the Monkey", "Mend Pet", "Aspect of the Cheetah", "Track Beasts", "Track Humanoids", "Track Undead", "Track Hidden", "Trueshot Aura", "Quick Shots", "Feed Pet", "Feed Pet Effect", "Call Pet", "Tame Beast", "Revive Pet"},
	"ROGUE":   {"Sinister Strike", "Eviscerate", "Backstab", "Gouge", "Kick", "Slice and Dice", "Ambush", "Garrote", "Rupture", "Hemorrhage", "Ghostly Strike", "Riposte", "Kidney Shot", "Cheap Shot", "Sap", "Stealth", "Evasion", "Sprint", "Expose Armor", "Deadly Poison", "Instant Poison"},
	"PRIEST":  {"Smite", "Mind Blast", "Shadow Word: Pain", "Lesser Heal", "Power Word: Shield", "Renew", "Holy Fire", "Mind Flay", "Flash Heal", "Heal", "Greater Heal", "Psychic Scream", "Power Word: Fortitude", "Inner Fire", "Devouring Plague", "Vampiric Embrace", "Divine Spirit", "Weakened Soul", "Shadowform", "Shadow Protection", "Spirit Tap", "Prayer of Fortitude"},
	"SHAMAN":  {"Lightning Bolt", "Earth Shock", "Flame Shock", "Frost Shock", "Healing Wave", "Lesser Healing Wave", "Chain Lightning", "Lightning Shield", "Rockbiter Weapon", "Flametongue Weapon", "Flametongue Attack", "Stormstrike", "Searing Totem", "Magma Totem", "Stoneskin Totem", "Strength of Earth Totem", "Chain Heal", "Fire Nova", "Ghost Wolf", "Water Shield"},
	"MAGE":    {"Fireball", "Frostbolt", "Fire Blast", "Arcane Missiles", "Arcane Missile", "Arcane Explosion", "Frost Nova", "Flamestrike", "Blizzard", "Cone of Cold", "Scorch", "Pyroblast", "Arcane Intellect", "Frost Armor", "Ice Armor", "Polymorph", "Blink", "Conjure Water", "Conjure Food", "Ice Barrier", "Mana Shield", "Mage Armor", "Dampen Magic", "Missile Barrage", "Arcane Blast"},
	"WARLOCK": {"Shadow Bolt", "Corruption", "Immolate", "Bane of Agony", "Curse of Agony", "Drain Life", "Drain Soul", "Life Tap", "Searing Pain", "Shadowburn", "Hellfire", "Rain of Fire", "Death Coil", "Fear", "Howl of Terror", "Siphon Life", "Demon Skin", "Demon Armor", "Summon Imp", "Summon Voidwalker", "Health Funnel", "Soul Fire", "Curse of Weakness", "Unending Breath", "Summon Felsteed", "Soulstone Resurrection", "Sense Demons", "Create Healthstone"},
	"DRUID":   {"Wrath", "Moonfire", "Starfire", "Healing Touch", "Rejuvenation", "Regrowth", "Entangling Roots", "Thorns", "Mark of the Wild", "Insect Swarm", "Hurricane", "Maul", "Swipe", "Claw", "Rake", "Shred", "Ferocious Bite", "Rip", "Bear Form", "Cat Form", "Demoralizing Roar", "Bash", "Faerie Fire", "Enrage", "Travel Form", "Leader of the Pack", "Furor"},
}

var combatClassByName = func() map[string]int {
	out := map[string]int{}
	for class, names := range combatClassSpellNames {
		for _, name := range names {
			out[name] = combatClassIDs[class]
		}
	}
	return out
}()

var combatEvidenceEvents = map[string]bool{
	"SPELL_CAST_START": true, "SPELL_CAST_SUCCESS": true, "SPELL_DAMAGE": true, "SPELL_PERIODIC_DAMAGE": true,
	"SPELL_HEAL": true, "SPELL_PERIODIC_HEAL": true, "SPELL_AURA_APPLIED": true,
}

type combatPlayers struct {
	index  map[string]int
	guids  []string
	names  []string
	masks  []uint32
	census combatCensus
}

func newCombatPlayers(census combatCensus) *combatPlayers {
	return &combatPlayers{index: map[string]int{}, census: census}
}

func (p *combatPlayers) id(guid string) int {
	if i, ok := p.index[guid]; ok {
		return i
	}
	i := len(p.names)
	p.index[guid] = i
	p.guids = append(p.guids, guid)
	p.names = append(p.names, "")
	p.masks = append(p.masks, 0)
	return i
}

func (p *combatPlayers) token(guid string) string {
	if p == nil {
		return anonymousPlayer
	}
	return string(combatTokenStart) + strconv.Itoa(p.id(guid)) + string(combatTokenEnd)
}

func combatUnquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func (p *combatPlayers) observe(f []string) {
	if p == nil || len(f) < 9 {
		return
	}
	for _, i := range []int{1, 5} {
		if combatPlayerRe.MatchString(f[i]) {
			if name := combatUnquote(f[i+1]); name != "" && name != "nil" {
				p.names[p.id(f[i])] = name
			}
		}
	}
	if len(f) < 11 || !combatEvidenceEvents[f[0]] || !combatPlayerRe.MatchString(f[1]) {
		return
	}
	name := combatUnquote(f[10])
	class := combatClassByName[name]
	if id, err := strconv.ParseUint(f[9], 10, 64); err == nil {
		if c, ok := combatClassCasts[combatClassSpell{id, name}]; ok {
			class = combatClassIDs[c]
		}
	}
	if class > 0 {
		i := p.id(f[1])
		p.masks[i] |= 1 << class
	}
}

func (p *combatPlayers) class(i int) int {
	mask := p.masks[i]
	census := p.census.lookup(p.names[i], combatGUIDRealm(p.guids[i]))
	switch {
	case census > 0 && (mask == 0 || mask == 1<<census):
		return census
	case census == 0 && bits.OnesCount32(mask) == 1:
		return bits.TrailingZeros32(mask)
	}
	return 0
}

func combatClassGUID(class int) string {
	if class <= 0 {
		return anonymousPlayer
	}
	return fmt.Sprintf("Player-0-%08X", class)
}

func (p *combatPlayers) expand(src []byte) []byte {
	tags := make([]string, len(p.names))
	for i := range tags {
		tags[i] = combatClassGUID(p.class(i))
	}
	var out bytes.Buffer
	out.Grow(len(src))
	for {
		start := bytes.IndexByte(src, combatTokenStart)
		if start < 0 {
			out.Write(src)
			return out.Bytes()
		}
		end := bytes.IndexByte(src[start:], combatTokenEnd)
		i, err := strconv.Atoi(string(src[start+1 : start+end]))
		out.Write(src[:start])
		if err != nil || i < 0 || i >= len(tags) {
			out.WriteString(anonymousPlayer)
		} else {
			out.WriteString(tags[i])
		}
		src = src[start+end+1:]
	}
}

func combatPrivateGUID(guid string) bool {
	return guid == anonymousPlayer || guid == anonymousPet || strings.HasPrefix(guid, string(combatTokenStart))
}

type combatCensus map[string]int

func combatGUIDRealm(guid string) string {
	parts := strings.Split(guid, "-")
	if len(parts) != 3 || parts[1] == "0" {
		return ""
	}
	return parts[1]
}

func combatRealmKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c combatCensus) add(key, classFile, realmID string) {
	class := combatClassIDs[strings.ToUpper(classFile)]
	parts := strings.SplitN(key, ":", 4)
	if class == 0 || len(parts) != 4 || parts[0] == "" || parts[3] == "" {
		return
	}
	name := strings.ToLower(parts[3])
	keys := []string{parts[0] + "|" + name, parts[0] + "|" + combatRealmKey(parts[2]) + "|" + name}
	if realmID != "" {
		keys = append(keys, "#"+realmID+"|"+name)
	}
	for _, k := range keys {
		if old, ok := c[k]; ok && old != class {
			c[k] = -1
		} else {
			c[k] = class
		}
	}
}

func (c combatCensus) lookup(logName, realmID string) int {
	parts := strings.Split(logName, "-")
	if len(c) == 0 || len(parts) < 3 {
		return 0
	}
	region := strings.ToLower(parts[len(parts)-1])
	var key string
	if len(parts) == 3 {
		key = region + "|" + combatRealmKey(parts[1]) + "|" + strings.ToLower(parts[0])
	} else {
		name := strings.ToLower(strings.Join(parts[:len(parts)-2], " "))
		key = region + "|" + name
		if class, ok := c["#"+realmID+"|"+name]; ok && realmID != "" {
			return max(class, 0)
		}
	}
	if class := c[key]; class > 0 {
		return class
	}
	return 0
}

func loadCombatCensus(edDir string) combatCensus {
	out := combatCensus{}
	for _, f := range savedVarsInEdition(edDir) {
		src, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		out.addSavedVariables(src)
	}
	return out
}

func (c combatCensus) addSavedVariables(src []byte) {
	assigns, err := parseSavedVariables(src)
	if err != nil {
		return
	}
	for _, a := range assigns {
		if a.Name != "WoWCensusDB" || a.Value.Field("realms") == nil {
			continue
		}
		for _, realm := range a.Value.Field("realms").Entries {
			pop := realm.Value.Field("population")
			realmID := ""
			if n := realm.Value.Field("identity").Field("realmID").Number(); n > 0 {
				realmID = strconv.FormatFloat(n, 'f', 0, 64)
			}
			if chars := pop.Field("characters"); chars != nil {
				for _, e := range chars.Entries {
					c.add(e.KeyString(), e.Value.Field("classFile").String(), realmID)
				}
			}
			if packed, ok := pop.Field("charactersPacked").Array(); ok && len(packed) > 0 && packed[0].Value.String() == "MLC1" {
				for _, chunk := range packed[1:] {
					c.addPacked(chunk.Value.String(), realmID)
				}
			}
		}
	}
}

func (c combatCensus) addPacked(chunk, realmID string) {
	const fieldsPerRecord, classField = 16, 10
	var record []string
	for chunk != "" {
		colon := strings.IndexByte(chunk, ':')
		if colon < 0 {
			return
		}
		size, err := strconv.Atoi(chunk[:colon])
		if err != nil || size < 0 || colon+1+size > len(chunk) {
			return
		}
		record = append(record, chunk[colon+1:colon+1+size])
		chunk = chunk[colon+1+size:]
		if len(record) == fieldsPerRecord {
			c.add(record[0], record[classField], realmID)
			record = record[:0]
		}
	}
}
