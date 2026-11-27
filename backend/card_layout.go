package main

import (
	"regexp"
	"unicode/utf8"
)

var cardColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type cardLayout struct {
	Version         int        `json:"version"`
	BackgroundImage *int       `json:"backgroundImage,omitempty"`
	Items           []cardItem `json:"items"`
}
type cardItem struct {
	Type     string  `json:"type"`
	Text     string  `json:"text,omitempty"`
	Image    int     `json:"image,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Rotation float64 `json:"rotation"`
	Font     string  `json:"font,omitempty"`
	Size     float64 `json:"size,omitempty"`
	Color    string  `json:"color,omitempty"`
}

func validCardLayout(layout cardLayout, images int) bool {
	if layout.Version != 1 || len(layout.Items) > 24 {
		return false
	}
	if layout.BackgroundImage != nil && (*layout.BackgroundImage < 0 || *layout.BackgroundImage >= images) {
		return false
	}
	chars := 0
	for _, item := range layout.Items {
		if item.X < 0 || item.X > 100 || item.Y < 0 || item.Y > 100 || item.Width < 5 || item.Width > 100 || item.Rotation < -180 || item.Rotation > 360 {
			return false
		}
		switch item.Type {
		case "text":
			chars += utf8.RuneCountInString(item.Text)
			if item.Size < 10 || item.Size > 72 || (item.Color != "" && !cardColor.MatchString(item.Color)) || (item.Font != "sans-serif" && item.Font != "serif" && item.Font != "monospace") {
				return false
			}
		case "image":
			if item.Image < 0 || item.Image >= images {
				return false
			}
		default:
			return false
		}
	}
	return chars <= 280
}
