package model

import "strings"

type User struct {
	ID       int64    `json:"-"`
	Gender   string   `json:"gender"`
	Name     Name     `json:"name"`
	Location Location `json:"location"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Cell     string   `json:"cell"`
	Picture  Picture  `json:"picture"`
	Nat      string   `json:"nat"`
}

type Name struct {
	Title string `json:"title"`
	First string `json:"first"`
	Last  string `json:"last"`
}

type Location struct {
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	Postcode string `json:"postcode"`
}

type Picture struct {
	Large     string `json:"large"`
	Medium    string `json:"medium"`
	Thumbnail string `json:"thumbnail"`
}

func (u User) FullName() string {
	return strings.TrimSpace(u.Name.Title + " " + u.Name.First + " " + u.Name.Last)
}
