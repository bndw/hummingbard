package client

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func (c *Client) LinkMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		user, err := c.GetTokenUser(token)
		if err != nil || user == nil {
			log.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type payload struct {
			Href string `json:"href"`
		}

		var pay payload
		if r.Body == nil {
			log.Println(err)
			http.Error(w, "Please send a request body", 400)
			return
		}
		err = json.NewDecoder(r.Body).Decode(&pay)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), 400)
			return
		}

		log.Println("recieved payload ", pay)

		type Response struct {
			Title       string `json:"title,omitempty"`
			Author      string `json:"author,omitempty"`
			Description string `json:"description,omitempty"`
			Image       string `json:"image,omitempty"`
			IsYoutube   bool   `json:"is_youtube,omitempty"`
			YoutubeID   string `json:"youtube_id,omitempty"`
		}

		ff := Response{}

		up, err := url.Parse(pay.Href)
		if err != nil || up == nil || up.Host == "" {
			http.Error(w, err.Error(), 400)
			return
		}

		isYoutube := up.Host == "www.youtube.com" || up.Host == "youtube.com"
		isShortYoutube := up.Host == "youtu.be"

		var title, description, image, author string

		if isYoutube || isShortYoutube {

			m, err := url.ParseQuery(up.RawQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			id := ""

			if isYoutube {

				if len(m) > 0 {
					id = m["v"][0]
				}
			} else if isShortYoutube {

				if isShortYoutube {
					id = up.Path[1:]
				}
			}

			key := c.Config.YoutubeKey

			ctx := context.Background()
			ctx, _ = context.WithTimeout(ctx, 7*time.Second)
			service, err := youtube.NewService(ctx, option.WithAPIKey(key))
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			videos := service.Videos.List([]string{"id", "snippet"})

			videos = videos.Id(id)

			response, err := videos.Do()

			if err != nil {
				log.Println(err)
			}

			if response != nil {

				items := response.Items

				if items != nil && len(items) >= 1 {
					title = items[0].Snippet.Title
					description = items[0].Snippet.Description
				}

				if len(description) > 500 {
					description = description[:500]
				}

				ff.IsYoutube = true
				ff.YoutubeID = id
			}

		} else {
			md := c.Scrape(pay.Href, up.Host)

			title = md.Title
			description = md.Description
			author = md.Author
			image = md.Image
		}

		ff.Title = title
		ff.Author = author
		ff.Description = description
		ff.Image = image

		js, err := json.Marshal(ff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)

	}
}

type LinkMetaData struct {
	Title       string
	Description string
	Image       string
	Author      string
}

func (c *Client) Scrape(link string, domain string) LinkMetaData {
	co := colly.NewCollector()
	extensions.RandomUserAgent(co)
	extensions.Referer(co)

	htmldata := ""
	lmd := LinkMetaData{}

	co.OnResponse(func(r *colly.Response) {
		htmldata = string(r.Body)
	})

	co.OnHTML("head", func(e *colly.HTMLElement) {

		// Extract meta tags from the document
		metaTags := e.DOM.ParentsUntil("~").Find("meta")

		metaTags.Each(func(_ int, s *goquery.Selection) {
			// Search for og:type meta tags
			name, _ := s.Attr("name")
			prop, _ := s.Attr("property")

			if strings.EqualFold(name, "description") {
				description, _ := s.Attr("content")
				lmd.Description = description
			}

			if strings.EqualFold(name, "author") {
				author, _ := s.Attr("content")
				lmd.Author = author
			}

			if strings.EqualFold(prop, "og:image") {
				image, _ := s.Attr("content")
				lmd.Image = image
			}

			if lmd.Image == "" && strings.EqualFold(prop, "twitter:image:src") {
				image, _ := s.Attr("content")
				lmd.Image = image
			}

		})

	})

	co.OnHTML("head title", func(e *colly.HTMLElement) {
		lmd.Title = e.Text

	})

	co.OnRequest(func(r *colly.Request) {
	})

	co.Visit(link)

	return lmd
}
