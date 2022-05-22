package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"strconv"
	"flag"
	"time"
	"io/ioutil"
	"os"
	"os/user"
	"net/url"

	"github.com/ChimeraCoder/anaconda"
)

var exitcode int = 0
var next_max int64 = 0
var next_since int64 = 0
func print_id() {
	fmt.Fprintf(os.Stderr, "--------\n-since_id=%d\n", next_since)
	fmt.Fprintf(os.Stderr,   "-max_id=%d\n", next_max)
}

const onetime = 200
const sleepdot = 5

// TL type "enum"
type tltype int
const (
	tluser tltype = iota
	tlhome
	tlmention
	tlrtofme
)

type revtype bool
const (
	reverse revtype = true
	forward revtype = false
)

func sleep(second int64) {
	fmt.Fprintf(os.Stderr, "%s Sleep: %d", time.Now().Format("15:04:05"), second)
	start := time.Now()
	startunix := start.Unix()
	lastunix := startunix + int64(second)
	
	for second > 0 {
		slp := second
		if slp > sleepdot {
			slp = sleepdot
		}
		
		time.Sleep(time.Duration(slp) * time.Second)
		fmt.Fprintf(os.Stderr, ".")

		now := time.Now()
		nowunix := now.Unix()
		second = lastunix - nowunix
		if second < -10 {
			fmt.Fprintf(os.Stderr, "oversleep %s\n", now.Format("15:04:05"))
			print_id()
			os.Exit(0)
		}
		if second <= 0 {
			break
		}
	}
}

func getTL(t tltype, uv url.Values) (tweets []anaconda.Tweet, err error) {
	switch t {
	case tluser:
		tweets, err = api.GetUserTimeline(uv)
	case tlhome:
		tweets, err = api.GetHomeTimeline(uv)
	case tlmention:
		tweets, err = api.GetMentionsTimeline(uv)
	case tlrtofme:
		tweets, err = api.GetRetweetsOfMe(uv)
	}
	fmt.Fprintf(os.Stderr, "%s get len: %d\n", time.Now().Format("15:04:05"), len(tweets))
	return tweets, err
}

var api *anaconda.TwitterApi
func main(){
	tLtypePtr := flag.String("get", "user", "TLtype: user, home, mention, rtofme")
	countPtr := flag.Int("count", 0, "tweet count. max=3191?")
	max_idPtr := flag.Int64("max_id", 0, "starting tweet id")
	since_idPtr := flag.Int64("since_id", 0, "reverse start tweet id")
	screennamePtr := flag.String("user", "", "twitter @ screenname")
	idPtr  := flag.String("id", "", "integer user Id")
	reversePtr := flag.Bool("reverse", false, "reverse output. wait newest TL")
	loopsPtr := flag.Int("loops", 0, "API get loop max")
	waitPtr := flag.Int64("wait", 0, "wait second for next loop")
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "positional argument no need [%s]\n", flag.Arg(0))
		os.Exit(2)
	}
	
	api = connectTwitterApi()
	var uv=url.Values{}

	var t tltype
	switch *tLtypePtr {
	case "user":
		t = tluser
	case "home":
		t = tlhome
		if *idPtr != "" || *screennamePtr != "" {
			fmt.Fprintln(os.Stderr, "home TL for Auth user only.")
			os.Exit(2)
		}
	case "mention":
		t = tlmention
		if *idPtr != "" || *screennamePtr != "" {
			fmt.Fprintln(os.Stderr, "mention TL to Auth user only.")
			os.Exit(2)
		}
	case "rtofme":
		t = tlrtofme
		if *idPtr != "" || *screennamePtr != "" {
			fmt.Fprintln(os.Stderr, "RTofme TL for Auth user only.")
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "invalid type -get=%s\n", *tLtypePtr)
		os.Exit(2)
	}

	if *idPtr != "" {
		uv.Set("id", *idPtr)
		fmt.Fprintf(os.Stderr, "user id=%s\n", *idPtr)
		if (*screennamePtr != "") {
			fmt.Fprintln(os.Stderr, "screen name ignored.")
		}
	} else if *screennamePtr != "" {
		uv.Set("screen_name", *screennamePtr)
	} else {
		fmt.Fprintln(os.Stderr, "Auth user's TL")
	}

	//uv.Set("trim_user", "true")  //userは id, id_str しか含まない。screen_name:""
	//uv.Set("include_rts", "false")  //リツイートは含まない。件数は減る。

	for key, val := range uv {
		fmt.Fprintln(os.Stderr, key, ":", val)
	}
	fmt.Fprintf(os.Stderr, "count=%d\n", *countPtr)
	fmt.Fprintf(os.Stderr, "loops=%d\n", *loopsPtr)
	fmt.Fprintf(os.Stderr, "max_id=%d\n", *max_idPtr)
	fmt.Fprintf(os.Stderr, "since_id=%d\n", *since_idPtr)
	fmt.Fprintf(os.Stderr, "wait=%d\n", *waitPtr)

	var count = *countPtr
	var waitsecond = *waitPtr
	if *reversePtr {
		if *max_idPtr != 0 {
			fmt.Fprintf(os.Stderr, "max id ignored when reverse\n")
		}
		if waitsecond <= 0 {
			waitsecond = 60
			fmt.Fprintf(os.Stderr, "wait default=%d (reverse)\n", waitsecond)
		}
		getReverseTLs(t, uv, count, *loopsPtr, waitsecond, *since_idPtr)
	} else {
		if *loopsPtr == 0 && count == 0 {
			count = 5
			fmt.Fprintf(os.Stderr, "set forward default count=5\n")
		}
		if *max_idPtr > 0 && *max_idPtr <= *since_idPtr {
			fmt.Fprintf(os.Stderr, "sincd id ignored when max<=since\n")
		}
		if waitsecond <= 0 {
			waitsecond = 10
			fmt.Fprintf(os.Stderr, "wait default=%d (forward)\n", waitsecond)
		}
		getFowardTLs(t, uv, count, *loopsPtr, waitsecond, *max_idPtr, *since_idPtr)
	}
	print_id()
	os.Exit(exitcode)
}

func getFowardTLs(t tltype, uv url.Values, count int, loops int, wait int64, max int64, since int64) {
	var tweets []anaconda.Tweet
	var err error
	var countlim bool = true
	if count <=  0 {
		countlim = false
		uv.Set("count", strconv.Itoa(onetime))
	}
	if max > 0 {
		uv.Set("max_id", strconv.FormatInt(max - 1, 10))
		if max <= since {
			since = 0
		}
	}
	if since > 0 {
		uv.Set("since_id", strconv.FormatInt(since - 1, 10))
	}
	waitsecond := wait
	for i := 1; ; i++ {
		if countlim {
			c := count
			if c > onetime {
				c = onetime
			}
			uv.Set("count", strconv.Itoa(c))
		}
		tweets, err = getTL(t, uv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			print_id()
			os.Exit(2)
		}
		// jsonTweets, _ := json.Marshal(tweets) //test
		// fmt.Println(string(jsonTweets))       //test
		
		c := len(tweets)
		if c == 0 {
			exitcode = 1
			break
		}

		firstid, lastid := printTL(tweets, forward)
		if next_since == 0 {
			next_since = firstid
		}
		next_max = lastid
		
		if lastid <= since {
			break
		}
		if countlim {
			count -= c
			if count <= 0 { break }
		}
		if loops > 0 && i >= loops {
			break
		}
		uv.Set("max_id", strconv.FormatInt(lastid - 1, 10))

		sleep(waitsecond) //?
	}
	return
}

func getReverseTLs(t tltype, uv url.Values, count int, loops int, wait int64, since int64) {
	var tweets []anaconda.Tweet
	var err error
	var zeror bool
	var countlim bool = true
	if count <=  0 {
		countlim = false
	}
	waitsecond := wait
	var sinceid int64 = since
	var zerocount int = 0
	const maxzero int = 1
	next_since = sinceid //default: same sinceid

	if sinceid <= 0 {
		fmt.Fprintf(os.Stderr, "since=%d. get one tweet\n", sinceid)
		uv.Set("count", "1")
		tweets, err = getTL(t, uv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			print_id()
			os.Exit(2)
		}
		c := len(tweets)
		if c == 0 {
			fmt.Fprintln(os.Stderr, "Not 1 record available")
			os.Exit(2)
		}

		_, lastid := printTL(tweets[0:1], reverse)
		next_max = lastid
		next_since = lastid
		sinceid = lastid
		sleep(5)
	} else {
		fmt.Fprintf(os.Stderr, "since=%d. start from this record.\n", sinceid)
	}
	for i:=1; ; i+=1 {
		tweets, zeror = getTLsince(t, uv, sinceid)

		c := len(tweets)
		if c > 0 {
			zerocount = 0
			minid := tweets[len(tweets) - 1].Id
			if minid <= sinceid {
				//指定ツイートまで取れたのでダブらないように削除する
				tweets = tweets[: len(tweets) - 1]
				c = len(tweets)
			} else {
				//gap
				fmt.Fprintf(os.Stderr, "Gap exists\n")
			}
			if c > 0 {
				firstid, lastid := printTL(tweets, reverse)
				if next_max == 0 {
					next_max = firstid
				}
				next_since = lastid
				sinceid = lastid
				if countlim {
					count -= c
					if count <= 0 { break }
				}
			}
			if zeror {
				//accident. no response
				zerocount += 1
				if zerocount == maxzero {
					exitcode = 1
					break
				}
			}
		} else {
			//accident. no response
			zerocount += 1
			if zerocount == maxzero {
				exitcode = 1
				break
			}
		}
		if loops > 0 && i >= loops {
			break
		}
		sleep(waitsecond)
	}
	return
}

func getTLsince(t tltype, uv url.Values, since int64) (tweets []anaconda.Tweet, zeror bool) {
	tweets = []anaconda.Tweet{}
	zeror = false
	var tws []anaconda.Tweet
	var err error
	uv.Set("count", strconv.Itoa(onetime))
	uv.Set("since_id", strconv.FormatInt(since - 1, 10))
	for i := 0; ; i++ {

		tws, err = getTL(t, uv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			print_id()
			os.Exit(2)
		}
		c := len(tws)
		if c == 0 {
			zeror = true
			break
		}

		lastid := tws[c - 1].Id

		tweets = append(tweets, tws...)

		if lastid <= since {
			break
		}
		// 一度で取りきれなかった
		fmt.Fprintln(os.Stderr, "------continue")
		uv.Set("max_id", strconv.FormatInt(lastid - 1, 10))

		sleep(10) //??
	}
	return tweets, zeror
}

func printTL(tweets []anaconda.Tweet, revs revtype) (firstid int64, lastid int64) {
	firstid = 0
	lastid = 0
	imax := len(tweets)
	is := 0
	ip := 1
	if revs {
		is = imax - 1
		ip = -1
	}
	for i := is; 0 <= i && i < imax; i += ip {
		tweet := tweets[i]
		id := tweet.Id
		fmt.Fprintln(os.Stderr, "Id:", id)

		if i == is {
			firstid = id
			lastid = id
		}
		//  RT > Reply > Mention > tweet
		twtype := "tw"
		if tweet.InReplyToUserID != 0 {
			twtype = "Mn"
		}
		if tweet.InReplyToStatusID != 0 {
			twtype = "Re"
		}
		if tweet.RetweetedStatus != nil {
			twtype = "RT"
			printTweet(id, *tweet.RetweetedStatus, twtype)
		} else {
			printTweet(id, tweet, twtype)
		}
		lastid = id
	}
	return firstid, lastid
}

func printTweet(id int64, tweet anaconda.Tweet, twtype string) {
	screen := tweet.User.ScreenName
	fulltext := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(tweet.FullText, "\n", `\n`), "\r", `\r`), "\"", `\"`)
	fulltext = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(fulltext, `&amp;`, `&`), `&lt;`, `<`), `&gt;`, `>`)
	fmt.Printf("%d\t@%s\t%s\t\"%s\"\n", id, screen, twtype, fulltext)
}

func connectTwitterApi() *anaconda.TwitterApi {
	usr, _ := user.Current()
	raw, error := ioutil.ReadFile(usr.HomeDir + "/twitter/twitterAccount.json")

	if error != nil {
		fmt.Fprintln(os.Stderr, error.Error())
		return nil
	}

	var twitterAccount TwitterAccount
	json.Unmarshal(raw, &twitterAccount)

	return anaconda.NewTwitterApiWithCredentials(
		twitterAccount.AccessToken,
		twitterAccount.AccessTokenSecret,
		twitterAccount.ConsumerKey,
		twitterAccount.ConsumerSecret)

}

type TwitterAccount struct {
	AccessToken string `json:"accessToken"`
	AccessTokenSecret string `json:"accessTokenSecret"`
	ConsumerKey string `json:"consumerKey"`
	ConsumerSecret string `json:"consumerSecret"`
}

//type Tweet struct {
// 	Contributors                []int64                `json:"contributors"`
// 	Coordinates                 *Coordinates           `json:"coordinates"`
// 	CreatedAt                   string                 `json:"created_at"`
// 	DisplayTextRange            []int                  `json:"display_text_range"`
// 	Entities                    Entities               `json:"entities"`
// 	ExtendedEntities            Entities               `json:"extended_entities"`
// 	ExtendedTweet               ExtendedTweet          `json:"extended_tweet"`
// 	FavoriteCount               int                    `json:"favorite_count"`
// 	Favorited                   bool                   `json:"favorited"`
// 	FilterLevel                 string                 `json:"filter_level"`
// 	FullText                    string                 `json:"full_text"`
// 	HasExtendedProfile          bool                   `json:"has_extended_profile"`
// 	Id                          int64                  `json:"id"`
// 	IdStr                       string                 `json:"id_str"`
// 	InReplyToScreenName         string                 `json:"in_reply_to_screen_name"`
// 	InReplyToStatusID           int64                  `json:"in_reply_to_status_id"`
// 	InReplyToStatusIdStr        string                 `json:"in_reply_to_status_id_str"`
// 	InReplyToUserID             int64                  `json:"in_reply_to_user_id"`
// 	InReplyToUserIdStr          string                 `json:"in_reply_to_user_id_str"`
// 	IsTranslationEnabled        bool                   `json:"is_translation_enabled"`
// 	Lang                        string                 `json:"lang"`
// 	Place                       Place                  `json:"place"`
// 	QuotedStatusID              int64                  `json:"quoted_status_id"`
// 	QuotedStatusIdStr           string                 `json:"quoted_status_id_str"`
// 	QuotedStatus                *Tweet                 `json:"quoted_status"`
// 	PossiblySensitive           bool                   `json:"possibly_sensitive"`
// 	PossiblySensitiveAppealable bool                   `json:"possibly_sensitive_appealable"`
// 	RetweetCount                int                    `json:"retweet_count"`
// 	Retweeted                   bool                   `json:"retweeted"`
// 	RetweetedStatus             *Tweet                 `json:"retweeted_status"`
// 	Source                      string                 `json:"source"`
// 	Scopes                      map[string]interface{} `json:"scopes"`
// 	Text                        string                 `json:"text"`
// 	User                        User                   `json:"user"`
// 	WithheldCopyright           bool                   `json:"withheld_copyright"`
// 	WithheldInCountries         []string               `json:"withheld_in_countries"`
// 	WithheldScope               string                 `json:"withheld_scope"`
//}

//type User struct {
// 	ContributorsEnabled            bool     `json:"contributors_enabled"`
// 	CreatedAt                      string   `json:"created_at"`
// 	DefaultProfile                 bool     `json:"default_profile"`
// 	DefaultProfileImage            bool     `json:"default_profile_image"`
// 	Description                    string   `json:"description"`
// 	Email                          string   `json:"email"`
// 	Entities                       Entities `json:"entities"`
// 	FavouritesCount                int      `json:"favourites_count"`
// 	FollowRequestSent              bool     `json:"follow_request_sent"`
// 	FollowersCount                 int      `json:"followers_count"`
// 	Following                      bool     `json:"following"`
// 	FriendsCount                   int      `json:"friends_count"`
// 	GeoEnabled                     bool     `json:"geo_enabled"`
// 	HasExtendedProfile             bool     `json:"has_extended_profile"`
// 	Id                             int64    `json:"id"`
// 	IdStr                          string   `json:"id_str"`
// 	IsTranslator                   bool     `json:"is_translator"`
// 	IsTranslationEnabled           bool     `json:"is_translation_enabled"`
// 	Lang                           string   `json:"lang"` // BCP-47 code of user defined language
// 	ListedCount                    int64    `json:"listed_count"`
// 	Location                       string   `json:"location"` // User defined location
// 	Name                           string   `json:"name"`
// 	Notifications                  bool     `json:"notifications"`
// 	ProfileBackgroundColor         string   `json:"profile_background_color"`
// 	ProfileBackgroundImageURL      string   `json:"profile_background_image_url"`
// 	ProfileBackgroundImageUrlHttps string   `json:"profile_background_image_url_https"`
// 	ProfileBackgroundTile          bool     `json:"profile_background_tile"`
// 	ProfileBannerURL               string   `json:"profile_banner_url"`
// 	ProfileImageURL                string   `json:"profile_image_url"`
// 	ProfileImageUrlHttps           string   `json:"profile_image_url_https"`
// 	ProfileLinkColor               string   `json:"profile_link_color"`
// 	ProfileSidebarBorderColor      string   `json:"profile_sidebar_border_color"`
// 	ProfileSidebarFillColor        string   `json:"profile_sidebar_fill_color"`
// 	ProfileTextColor               string   `json:"profile_text_color"`
// 	ProfileUseBackgroundImage      bool     `json:"profile_use_background_image"`
// 	Protected                      bool     `json:"protected"`
// 	ScreenName                     string   `json:"screen_name"`
// 	ShowAllInlineMedia             bool     `json:"show_all_inline_media"`
// 	Status                         *Tweet   `json:"status"` // Only included if the user is a friend
// 	StatusesCount                  int64    `json:"statuses_count"`
// 	TimeZone                       string   `json:"time_zone"`
// 	URL                            string   `json:"url"`
// 	UtcOffset                      int      `json:"utc_offset"`
// 	Verified                       bool     `json:"verified"`
// 	WithheldInCountries            []string `json:"withheld_in_countries"`
// 	WithheldScope                  string   `json:"withheld_scope"`
//}

// func getTLcount(t tltype, uv url.Values, count int) (tweets []anaconda.Tweet) {
//  	tweets = []anaconda.Tweet{}
//  	var tws []anaconda.Tweet
//  	var err error
//  	for i := 0; ; i++ {
//  		c := count
//  		if c > onetime {
//  			c = onetime
//  		}
//  		uv.Set("count", strconv.Itoa(c))
//  
//  		tws, err = getTL(t, uv)
//  		if err != nil {
//  			fmt.Fprintln(os.Stderr, err.Error())
//  			print_id()
//  			os.Exit(2)
//  		}
//  		c = len(tws)
//  		if c == 0 { break }
//  
//  		tweets = append(tweets, tws...)
//  
//  		count -= c
//  		if count <= 0 { break }
//  
//  		lastid := tws[c - 1].Id
//  		uv.Set("max_id", strconv.FormatInt(lastid - 1, 10))
//  
//  		sleep(10) //??
//  	}
//  	return tweets
// }

// func name2id(screen_name string) (id int64) {
//  	var uv=url.Values{}
//  	uv.Set("skip_status", "true") //データ減少
//  	users, err := api.GetUsersLookup(screen_name, uv)
//  	if err != nil {
//  		fmt.Fprintln(os.Stderr, err.Error())
//  		os.Exit(2)
//  	}
//  	//jsonUser, _ := json.Marshal(users[0])
//  	//fmt.Println(string(jsonUser))
//  	//os.Exit(9)
//  	
//  	var userinfo anaconda.User = users[0]
//  
//  	id = userinfo.Id
//  	return id
// }
