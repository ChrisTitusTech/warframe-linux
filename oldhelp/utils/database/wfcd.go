package database

import (
	"context"
	"fmt"
)

var wfcdBaseURL = "https://raw.githubusercontent.com/WFCD/warframe-items/%s/data/json/%s"

type wfcdSource struct {
	file  string
	table string
}

func wfcdJSONURL(ref string, file string) string {
	return fmt.Sprintf(wfcdBaseURL, ref, file)
}

func FetchWFCDRaw(ctx context.Context, ref string, file string) ([]map[string]any, error) {
	return FetchRaw(ctx, wfcdJSONURL(ref, file))
}

const modsWFCDFile = "Mods.json"

const arcanesWFCDFile = "Arcanes.json"

var weaponsWFCDFiles = []string{
	"Primary.json",
	"Secondary.json",
	"Melee.json",
	"Arch-Gun.json",
	"Arch-Melee.json",
}

const warframesWFCDFile = "Warframes.json"

const archwingWFCDFile = "Archwing.json"

const sentinelsWFCDFile = "Sentinels.json"

var ducatWFCDSources = []wfcdSource{
	{warframesWFCDFile, "warframes"},
	{archwingWFCDFile, "archwing"},
	{sentinelsWFCDFile, "companions"},
}
