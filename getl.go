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
	"os/signal"
	"syscall"
	"net/url"

	"github.com/ChimeraCoder/anaconda"
)

var exitcode int = 0
var next_max int64 = 0
var next_since int64 = 0
func print_id() {
	if uniqid != nil {
		err := uniqid.write()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	}
	fmt.Fprintf(os.Stderr, "--------\n-since_id=%d\n", next_since)
	fmt.Fprintf(os.Stderr,   "-max_id=%d\n", next_max)
}

const t_onetimedefault = 20
const t_onetimemax = 200
const s_onetimedefault = 15
const s_onetimemax = 100
var onetimedefault = t_onetimedefault
var onetimemax = t_onetimemax
const sleepdot = 5

// TL type "enum"
type tltype int
const (
	tlnone tltype = iota
	tluser
	tlhome
	tlmention
	tlrtofme
	tllist
	tlsearch
)

type revtype bool
const (
	reverse revtype = true
	forward revtype = false
)

const (
	rsrecent string = "recent"
	rsmixed string = "mixed"
	rspopular string = "popular"
)

type idCheck map[int64]bool
var uniqid idCheck = nil

func (c idCheck) checkID(id int64) (exist bool) {
	if c[id] {
		return true
	} else {
		c[id] = true
		return false
	}
}

func (c idCheck) write() (err error) {
	bytes, _ := json.Marshal(c)
	err = ioutil.WriteFile("tempids.json", bytes, os.FileMode(0600))
	return err
}

func (c *idCheck) read() (err error) {
	*c = idCheck{}
	raw, err := ioutil.ReadFile("tempids.json")
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	json.Unmarshal(raw, c)
	return nil
}


type twSearchApi struct {
	api *anaconda.TwitterApi
	t  tltype
	sr anaconda.SearchResponse
	sec int
}

var twapi twSearchApi

func (ta *twSearchApi) getTL(uv url.Values) (tweets []anaconda.Tweet, err error) {
	api := ta.api
	switch ta.t {
	case tluser:
		tweets, err = api.GetUserTimeline(uv)
	case tlhome:
		tweets, err = api.GetHomeTimeline(uv)
	case tlmention:
		tweets, err = api.GetMentionsTimeline(uv)
	case tlrtofme:
		tweets, err = api.GetRetweetsOfMe(uv)
	case tllist:
		listid64, _ := strconv.ParseInt(uv.Get("list_id"), 10, 64)
		includeRT, _ := strconv.ParseBool(uv.Get("include_rts"))
		tweets, err = api.GetListTweets(listid64, includeRT, uv)
	case tlsearch:
		if ta.sec == 0 {
			query := uv.Get("query")
			ta.sr, err = api.GetSearch(query, uv)
		} else {
			ta.sr, err = ta.sr.GetNext(ta.api)
		}
		ta.sec++
		tweets = ta.sr.Statuses
	}
	
	fmt.Fprintf(os.Stderr, "%s get len: %d\n", time.Now().Format("15:04:05"), len(tweets))
	return tweets, err
}

func (ta *twSearchApi) rewindQuery() {
	ta.sec = 0
}

// func (a TwitterApi) GetListTweets(listID int64, includeRTs bool, v url.Values) (tweets []Tweet, err error) {
//  	v = cleanValues(v)
//  	v.Set("list_id", strconv.FormatInt(listID, 10))
//  	v.Set("include_rts", strconv.FormatBool(includeRTs))
//  
//  	response_ch := make(chan response)
//  	a.queryQueue <- query{a.baseUrl + "/lists/statuses.json", v, &tweets, _GET, response_ch}
//  	return tweets, (<-response_ch).err
// }

func main(){
	var err error
	tLtypePtr := flag.String("get", "", "TLtype: user, home, mention, rtofme, list, search")
	screennamePtr := flag.String("user", "", "twitter @ screenname")
	useridPtr := flag.Int64("userid", 0, "integer user Id")
	listnamePtr := flag.String("listname", "", "list name")
	listIDPtr := flag.Int64("listid", 0, "list ID")
	queryPtr := flag.String("query", "", "Query String")
	resulttypePtr := flag.String("restype", "", "result type: recent/mixed(default)/popular")
	countPtr := flag.Int("count", 0, "tweet count. max=800 ?")
	eachPtr := flag.Int("each", 0, "req count for each loop max=200")
	max_idPtr := flag.Int64("max_id", 0, "starting tweet id")
	since_idPtr := flag.Int64("since_id", 0, "reverse start tweet id")
	reversePtr := flag.Bool("reverse", false, "reverse output. wait newest TL")
	loopsPtr := flag.Int("loops", 0, "get loop max")
	waitPtr := flag.Int64("wait", 0, "wait second for next loop")
	nortPtr := flag.Bool("nort", false, "not include retweets")
	flag.Parse()
	var tLtype = *tLtypePtr
	var screenname = *screennamePtr
	var userid = *useridPtr
	var listname = *listnamePtr
	var listID = *listIDPtr
	var queryString = *queryPtr
	var resulttype = *resulttypePtr
	var count = *countPtr
	var eachcount = *eachPtr
	var max_id = *max_idPtr
	var since_id = *since_idPtr
	var reverseflag = *reversePtr
	var max_loop = *loopsPtr
	var waitsecond = *waitPtr
	var includeRTs = ! *nortPtr
	
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "positional argument no need [%s]\n", flag.Arg(0))
		os.Exit(2)
	}

	var t tltype
	switch tLtype {
	case "user":    t = tluser
	case "home":    t = tlhome
	case "mention": t = tlmention
	case "rtofme":  t = tlrtofme
	case "list":    t = tllist
	case "search":  t = tlsearch
	case "":
		if listID > 0 || listname != "" {
			t = tllist
			tLtype = "list"
			fmt.Fprintln(os.Stderr, "assume -get=list")
		} else if userid > 0 || screenname != "" {
			t = tluser
			tLtype = "user"
			fmt.Fprintln(os.Stderr, "assume -get=user")
		} else if queryString != "" {
			t = tlsearch
			tLtype = "search"
			fmt.Fprintln(os.Stderr, "assume -get=search")
		} else {
			t = tlhome
			tLtype = "home"
			fmt.Fprintln(os.Stderr, "assume -get=home")
		}
	default:
		fmt.Fprintf(os.Stderr, "invalid type -get=%s\n", tLtype)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "-get=%s\n", tLtype)
	
	twapi.api = connectTwitterApi()
	twapi.t = t

	var uv=url.Values{}

	if userid == 0 && screenname == "" {
		switch t {
		case tluser:
			fmt.Fprintln(os.Stderr, "Auth user's TL")
		case tlhome:
			fmt.Fprintln(os.Stderr, "Auth user's Home TL")
		case tllist:
			fmt.Fprintln(os.Stderr, "ListTL needs -user=username or -userid=99999999")
			os.Exit(2)
		}
	} else {	
		switch t {
		case tlhome: fallthrough
		case tlmention: fallthrough
		case tlrtofme:
			fmt.Fprintf(os.Stderr, "%s TL for Auth user only.\n", tLtype)
			os.Exit(2)
		}
	}

	if userid != 0 {
		uv.Set("id", strconv.FormatInt(userid, 10))
		fmt.Fprintf(os.Stderr, "user id=%d\n", userid)
		if (screenname != "") {
			fmt.Fprintln(os.Stderr, "screen name ignored.")
		}
	} else if screenname != "" {
		switch t {
		case tllist:
			userid, err = name2id(screenname)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(2)
			}
			uv.Set("id", strconv.FormatInt(userid, 10))
		default:
			uv.Set("screen_name", screenname)
		}
	}


	switch t {
	case tllist:
		if listID > 0 && listname != "" {
			fmt.Fprintln(os.Stderr, "list name ignored.")
			listname = ""
		}
		listID = listIDCheck(userid, listID, listname)
		if listID == 0 {
			os.Exit(2)
		}
		uv.Set("list_id", strconv.FormatInt(listID, 10))
	default:
		if listID != 0 || listname != "" {
			fmt.Fprintln(os.Stderr, "no need list ID, List name.")
			os.Exit(2)
		}
	}

	switch t {
	case tlsearch:
		if queryString == "" {
			fmt.Fprintln(os.Stderr, "-query not specified")
			os.Exit(2)
		}
		switch {
		case strings.HasPrefix(rsrecent, resulttype):
			resulttype = rsrecent
		case strings.HasPrefix(rsmixed, resulttype):
			resulttype = rsmixed
		case strings.HasPrefix(rspopular, resulttype):
			resulttype = rspopular
		default:
			fmt.Fprintf(os.Stderr, "invalid -restype=%s\n", resulttype)
			os.Exit(2)
		}
		onetimedefault = s_onetimedefault
		onetimemax = s_onetimemax
		err = uniqid.read()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		uv.Set("query", queryString)
		uv.Set("result_type", resulttype)
	default:
		if queryString != "" {
			fmt.Fprintf(os.Stderr, "-get=%s no need -query\n", tLtype)
			os.Exit(2)
		}
		if resulttype != "" {
			fmt.Fprintf(os.Stderr, "-get=%s no need -restype=%s\n", tLtype, resulttype)
			os.Exit(2)
		}
		uv.Set("include_rts", strconv.FormatBool(includeRTs))  //リツイートは含まない。件数は減る。
	}		
	

	for key, val := range uv {
		fmt.Fprintln(os.Stderr, key, ":", val)
	}
	fmt.Fprintf(os.Stderr, "count=%d\n", count)
	fmt.Fprintf(os.Stderr, "each=%d\n", eachcount)
	fmt.Fprintf(os.Stderr, "reverse=%v\n", reverseflag)
	fmt.Fprintf(os.Stderr, "loops=%d\n", max_loop)
	fmt.Fprintf(os.Stderr, "max_id=%d\n", max_id)
	fmt.Fprintf(os.Stderr, "since_id=%d\n", since_id)
	fmt.Fprintf(os.Stderr, "wait=%d\n", waitsecond)

	sgchn.sighandle()
	
	if reverseflag {
		if max_id != 0 {
			fmt.Fprintf(os.Stderr, "max id ignored when reverse\n")
		}
		if waitsecond <= 0 {
			waitsecond = 60
			fmt.Fprintf(os.Stderr, "wait default=%d (reverse)\n", waitsecond)
		}
		getReverseTLs(uv, count, max_loop, waitsecond, since_id)
	} else {
		if max_id > 0 && max_id <= since_id {
			fmt.Fprintf(os.Stderr, "sincd id ignored when max<=since\n")
			since_id = 0
		}
		if max_loop == 0 && since_id == 0 && count == 0 {
			count = 5
			fmt.Fprintf(os.Stderr, "set forward default count=5\n")
		}
		if waitsecond <= 0 {
			waitsecond = 10
			fmt.Fprintf(os.Stderr, "wait default=%d (forward)\n", waitsecond)
		}
		getFowardTLs(uv, count, eachcount, max_loop, waitsecond, max_id, since_id)
	}
	print_id()
	os.Exit(exitcode)
}

func listIDCheck(userID int64, listid int64, listname string) (returnID int64) {
	var uv=url.Values{}
	uv.Set("count", "100")
	lists, err := twapi.api.GetListsOwnedBy(userID, uv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	if len(lists) <= 0 {
		fmt.Fprintln(os.Stderr, "no list in this user.")
		return 0
	}
	matchcount := 0
	for _, list := range lists {
		if listid > 0 {
			if list.Id == listid {
				return list.Id
			}
		} else if listname != "" {
			if strings.HasPrefix(list.Name, listname) {
				returnID = list.Id
				fmt.Fprintln(os.Stderr, "listId: ", list.Id, " Name: ", list.Name)
				matchcount += 1
			}
		}
	}
	if matchcount == 1 {
		return returnID
	} else if matchcount > 1 {
		fmt.Fprintln(os.Stderr, "choose list id.")
	} else {
		if listid == 0 && listname == "" {
			fmt.Fprintln(os.Stderr, "need -listid or -listname.")
		} else {
			fmt.Fprintln(os.Stderr, "list id or list name unmatch.")
		}
		for _, list := range lists {
			fmt.Fprintln(os.Stderr, "listId: ", list.Id, " Name: ", list.Name)
		}
	}
	return 0
}

func getFowardTLs(uv url.Values, count int, eachcount int, loops int, waitsecond int64, max int64, since int64) {
	var tweets []anaconda.Tweet
	var err error
	var countlim bool = true
	if count <= 0 {
		countlim = false
	}
	if eachcount > 0 {
		uv.Set("count", strconv.Itoa(eachcount))
	} else {
		if count > onetimedefault {
			eachcount = count * 2
			if eachcount > onetimemax {
				eachcount = onetimemax
			}
			fmt.Fprintf(os.Stderr, "-each=%d assumed\n", eachcount)
			uv.Set("count", strconv.Itoa(eachcount))
		}
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
	for i := 1; ; i++ {

		tweets, err = twapi.getTL(uv)
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

		firstid, lastid, nout := printTL(tweets, count, forward)
		if next_since == 0 {
			next_since = firstid
		}
		next_max = lastid
		
		if lastid <= since {
			break
		}
		if countlim {
			count -= nout
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

func getReverseTLs(uv url.Values, count int, loops int, wait int64, since int64) {
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
		tweets, err = twapi.getTL(uv)
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

		_, lastid, _ := printTL(tweets[0:1], 0, reverse)
		next_max = lastid
		next_since = lastid
		sinceid = lastid
		sleep(5)
	} else {
		fmt.Fprintf(os.Stderr, "since=%d. start from this record.\n", sinceid)
	}
	for i:=1; ; i+=1 {
		tweets, zeror = getTLsince(uv, sinceid)

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
				firstid, lastid, nout := printTL(tweets, 0, reverse)
				if next_max == 0 {
					next_max = firstid
				}
				next_since = lastid
				sinceid = lastid
				if countlim {
					count -= nout
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

func getTLsince(uv url.Values, since int64) (tweets []anaconda.Tweet, zeror bool) {
	tweets = []anaconda.Tweet{}
	zeror = false
	var tws []anaconda.Tweet
	var err error
	uv.Set("count", strconv.Itoa(onetimemax))
	uv.Set("since_id", strconv.FormatInt(since - 1, 10))
	twapi.rewindQuery()
	for i := 0; ; i++ {

		tws, err = twapi.getTL(uv)
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

type sigrecv_chan chan os.Signal
var sgchn sigrecv_chan
func (s *sigrecv_chan) sighandle() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs,
		syscall.SIGINT,
		syscall.SIGTERM)

	done := make(chan os.Signal, 1)

	go func() {
		sig := <-sigs
		done <- sig
	}()
	*s = done
}

func (s sigrecv_chan) checksignal() (bool) {
	select {
	case sig := <-s:
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, sig)
		return true
	default:
	}
	return false
}

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
		
		//time.Sleep(time.Duration(slp) * time.Second)
		for i := int64(0); i <= slp; i++ {
			if sgchn.checksignal() {
				print_id()
				os.Exit(3)
			}
			if i < slp {
				time.Sleep(time.Second)
			}
		}
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

func printTL(tweets []anaconda.Tweet, count int, revs revtype) (firstid int64, lastid int64, nout int) {
	firstid = 0
	lastid = 0
	imax := len(tweets)
	is := 0
	ip := 1
	if revs {
		is = imax - 1
		ip = -1
	}
	nout = 0
	for i := is; 0 <= i && i < imax; i += ip {
		tweet := tweets[i]
		id := tweet.Id

		if i == is {
			firstid = id
			lastid = id
		}
		//  RT > Reply > Mention > tweet
		done := printTweet(&tweet, tweet.RetweetedStatus)
		if done {
			nout++
		}

		lastid = id
		
		if count > 0 && nout >= count {
			break
		}
	}
	return firstid, lastid, nout
}

func printTweet(tweet1 *anaconda.Tweet, tweet2 *anaconda.Tweet) (bool) {
	tweetid := tweet1.Id
	tweetuser := tweet1.User.ScreenName

	twtype1 := "or"
	tweet := tweet1
	if tweet2 != nil {
		twtype1 = "RT"
		tweet = tweet2
	}
	origiid := tweet.Id
	origiuser := tweet.User.ScreenName
	
	firstp := true
	idst := "*Id:"
	if uniqid != nil {
		if uniqid.checkID(origiid) {
			firstp = false
			idst = "_id:"
		}
	}

	if tweet2 == nil {
		fmt.Fprintln(os.Stderr, idst, tweetid)
	} else {
		fmt.Fprintln(os.Stderr, idst, tweetid, origiid)
	}

	if firstp {
		twtype2 := "tw"
		if tweet.InReplyToUserID != 0 {
			twtype2 = "Mn"
		}
		if tweet.InReplyToStatusID != 0 {
			twtype2 = "Re"
		}

		fulltext := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(tweet.FullText, "\n", `\n`), "\r", `\r`), "\"", `\"`)
		fulltext = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(fulltext, `&amp;`, `&`), `&lt;`, `<`), `&gt;`, `>`)
		fmt.Printf("%d\t@%s\t%s\t%d\t@%s\t%s\t\"%s\"\n",
			tweetid, tweetuser, twtype1,
			origiid, origiuser, twtype2, fulltext)
	}
	return firstp
}

func name2id(screen_name string) (id int64, err error) {
	var uv=url.Values{}
	uv.Set("skip_status", "true") //データ減少
	users, err := twapi.api.GetUsersLookup(screen_name, uv)
	if err != nil {
		return 0, err
	}
	//jsonUser, _ := json.Marshal(users[0])
	//fmt.Println(string(jsonUser))
	//os.Exit(9)
	
	var userinfo anaconda.User = users[0]

	id = userinfo.Id
	return id, nil
}

func connectTwitterApi() *anaconda.TwitterApi {
	usr, _ := user.Current()
	raw, error := ioutil.ReadFile(usr.HomeDir + "/twitter/twitterAccount.json")

	if error != nil {
		fmt.Fprintln(os.Stderr, error.Error())
		os.Exit(2)
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

//type SearchMetadata struct {
// 	CompletedIn   float32 `json:"completed_in"`
// 	MaxId         int64   `json:"max_id"`
// 	MaxIdString   string  `json:"max_id_str"`
// 	Query         string  `json:"query"`
// 	RefreshUrl    string  `json:"refresh_url"`
// 	Count         int     `json:"count"`
// 	SinceId       int64   `json:"since_id"`
// 	SinceIdString string  `json:"since_id_str"`
// 	NextResults   string  `json:"next_results"`
//}
// 
//type SearchResponse struct {
// 	Statuses []Tweet        `json:"statuses"`
// 	Metadata SearchMetadata `json:"search_metadata"`
//}
