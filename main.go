package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	"github.com/mattn/go-haiku"
	"github.com/robfig/cron"

	"github.com/ChimeraCoder/anaconda"
)

var accessToken = os.Getenv("accessToken")
var accessTokenSecret = os.Getenv("accessTokenSecret")
var consumerKey = os.Getenv("consumerKey")
var consumerSecret = os.Getenv("consumerSecret")

const senryuDetectedReplyText = "@%v 素晴らしい川柳ですね ＞＜;"
const tweetURL = "https://twitter.com/%v/status/%v"
const resultTweetText = "今週のトップ川柳はこれ！\n「%v %v %v」%v"
const senryuHashTag = "#川柳"

const resultTweetCronTime = "0 0 21 * * 0"

type SenryuTweet struct {
	ID             string `gorm:"primary_key" json:"id"`
	UserID         string `json:"user_id"`
	UserName       string `json: "user_name"`
	Text           string `json: "text"`
	FirstSentence  string `json: "first_sentence"`
	SecondSentence string `json: "second_sentence"`
	ThirdSentence  string `json: "third_sentence"`
	CreatedAt      time.Time
}

type SenryuTweets []SenryuTweet

// 文章中からハッシュタグを除去
func withoutHashTag(text string) string {
	rep := regexp.MustCompile(`[#＃][Ａ-Ｚａ-ｚA-Za-z一-鿆0-9０-９ぁ-ヶｦ-ﾟー]+`)
	return rep.ReplaceAllString(text, "")
}

// 文章中から川柳を上/中/下の句に分けて取得、川柳を取得できないとnilを返す
func getSenryu(text string) []string {
	finded := haiku.Find(text, []int{5, 7, 5})
	if len(finded) > 0 {
		sentenceSplited := strings.Fields(finded[0])
		return sentenceSplited
	} else {
		return nil
	}
}

// 一番ファボを集めた575ツイートを取得
func getTopFavoriteTweet(tweetApi *anaconda.TwitterApi, senryuTweets []SenryuTweet) (SenryuTweet, int, error) {
	topTw := SenryuTweet{}
	topFavoriteCount := -1
	isUpdated := false

	for _, stw := range senryuTweets {
		id, err := strconv.ParseInt(stw.ID, 10, 64)
		tw, err := tweetApi.GetTweet(id, url.Values{})
		if err != nil {
			continue
		}

		if topFavoriteCount < tw.FavoriteCount {
			topTw = stw
			topFavoriteCount = tw.FavoriteCount
			isUpdated = true
		}
	}

	if isUpdated {
		return topTw, topFavoriteCount, nil
	} else {
		return topTw, 0, errors.New("tweet not found")
	}
}

// 川柳ツイート検知時のリプライを送信
func postSenryuDetectedReply(tweetApi *anaconda.TwitterApi, tweetId string, userScreenName string) anaconda.Tweet {
	v := url.Values{}
	v.Add("in_reply_to_status_id", tweetId)
	tweetText := fmt.Sprintf(senryuDetectedReplyText, userScreenName)
	tweet, _ := tweetApi.PostTweet(tweetText, v)
	return tweet
}

// 今週のトップ川柳を発表
func postResultTweet(tweetApi *anaconda.TwitterApi, db *gorm.DB) (anaconda.Tweet, error) {
	tweets := []SenryuTweet{}
	db.Find(&tweets)

	topTweet, _, err := getTopFavoriteTweet(tweetApi, tweets)
	if err != nil {
		return anaconda.Tweet{}, err
	}

	topTweetURL := fmt.Sprintf(tweetURL, topTweet.UserName, topTweet.ID)
	postText := fmt.Sprintf(resultTweetText, topTweet.FirstSentence, topTweet.SecondSentence, topTweet.ThirdSentence, topTweetURL)

	v := url.Values{}
	postedTweet, err := tweetApi.PostTweet(postText, v)
	if err != nil {
		return anaconda.Tweet{}, err
	}

	db.Delete(&SenryuTweets{})
	return postedTweet, nil
}

func main() {
	// "user=uehr dbname=test sslmode=disable"
	db, err := gorm.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(&SenryuTweet{})
	defer db.Close()

	api := anaconda.NewTwitterApiWithCredentials(accessToken, accessTokenSecret, consumerKey, consumerSecret)

	// トップ川柳を発表するcron
	c := cron.New()
	c.AddFunc(resultTweetCronTime, func() {
		_, err := postResultTweet(api, db)
		if err != nil {
			panic(err)
		}
	})
	c.Start()

	v := url.Values{"track": []string{senryuHashTag}}
	s := api.PublicStreamFilter(v)

	for t := range s.C {
		switch v := t.(type) {
		case anaconda.Tweet:
			text := withoutHashTag(v.Text)
			text = strings.Replace(text, "\n", "", -1)
			text = strings.Replace(text, " ", "", -1)

			senryu := getSenryu(text)

			if senryu != nil {
				var tweetId = v.IdStr
				var userId = v.User.IdStr
				var text = v.Text
				var firstSentence = senryu[0]
				var secondSentence = senryu[1]
				var thirdSentence = senryu[2]
				var userScreenName = v.User.ScreenName

				var senryuTw = SenryuTweet{
					ID:             tweetId,
					UserID:         userId,
					UserName:       userScreenName,
					Text:           text,
					FirstSentence:  firstSentence,
					SecondSentence: secondSentence,
					ThirdSentence:  thirdSentence,
				}

				// 川柳ツイートを保存
				db.Create(&senryuTw)
				db.Save(&senryuTw)
			}
		}
	}
}
