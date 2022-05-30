# tw-timeline
Get twitter timeline command

~~~
$ ./getl -help
Usage of ./getl:
-get string
	TLtype: user, home, mention, rtofme, list

-user string
	twitter @ screenname
-userid int
	integer user Id

-listname string
	list name
-listid int
	list ID

-reverse
	reverse output. wait newest TL
-max_id int
	starting tweet id
-since_id int
	reverse start tweet id

-nort
	not include retweets
-count int
	tweet count. max=800?
-each int
	req count for each loop max=200
-loops int
	API get loop max
-wait int
	wait second for next loop
~~~


## parameter example
### 認証ユーザーのTL
    [-get=home]    (ホーム)  デフォルト
    -get=user      (自分のTL)
    -get=mention   (本人宛て)
    -get=rtofme    (RTされたもの)

### ユーザーTL
    [-get=user] -user=screenname / -userid=9999999
### リストTL
    [-get=list] -listname=リスト名 / -listid=99999999  -user=screenname / -userid=9999999

### 取得方向
    -reverse  (逆。最新待ち受け取得)  順方向は過去へ

### 続き指示
順方向ではこの次から古いものをとる

    -max_id=1529278564566454273

逆方向ではこの次から待ち受ける

    -since_id=1529278731545882624 -reverse

### その他パラメタ
    -count=取得件数めやす　　(デフォルトは順5件, 逆は制限なし。全体件数の制御)
    -each=一回の取得件数　 　(順のみ、デフォルト20件, 最大200件)
    -loops=内部繰り返し数　　(-countで全体件数を制御するか、-eachと-loops で制御してもよい)
    -wait=秒             　(ループ間隔)  デフォルト 順10 逆60
    -nort   　　　　　　　　(出力にRTを含めない)　　一回に取得できる件数が大幅に減るかもしれないので -each=200 を指定するとよい。
