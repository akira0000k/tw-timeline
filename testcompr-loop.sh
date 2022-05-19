#!/bin/bash
set -x
stdate=$(date +%Y%m%d-%H%M%S)
mkdir -p $stdate
stday=$(date +%Y%m%d)
filename=$stdate/$stday-home-r.tl
errname=$stdate/err-r.log
: > $filename
: > $errname
#backward
date

zeromax=2
zerocount=0
sinceid=
while :; do
    ./getl -get=home -reverse -loops=5 -wait=180  $sinceid 2>>$errname >>$filename
    ret=$?
    nextsince=$(tail -2 $errname | head -1)
    echo $nextsince
    if [[ "$nextsince" != '-since_id'* ]]; then
	break
    fi

    case "$ret" in
	0)  sleep 5;;
	1)  sleep 30;;
	*) break;;
    esac

    if [ "$nextsince" == '-since_id=0' ]; then
	: $(zerocount += 1)
	if [ $zerocount -ge $zeromax ]; then
	    break
	fi
    else
	zerocount=0
	sinceid=$nextsince
    fi
done
