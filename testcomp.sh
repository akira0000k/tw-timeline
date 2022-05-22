#!/bin/bash
set -x
#forward
date

stdate=$(date +%Y%m%d-%H%M%S)
mkdir -p $stdate
stday=$(date +%Y%m%d)
filename=$stdate/$stday-home-f.tl
errname=$stdate/err-f.log
: > $filename
: > $errname

maxid=
while :; do
    ./getl -get=home -loops=1 -count=200 $maxid 2>>$errname >>$filename
    ret=$?
    nextmax=$(tail -1 $errname)
    echo $nextmax
    if [[ "$nextmax" != '-max_id'* ]]; then
	break
    fi
    if [ "$nextmax" == '-max_id=0' ]; then
	break
    fi
    
    case "$ret" in
	0)  sleep 5;;
	1)  sleep 30;;
	*) break;;
    esac
    maxid=$nextmax
done
