#!/bin/bash
#
BINPATH="./"
if [ $# -ne 1 ]; then
	echo "building to ./webhook.arm"
fi
if [ $# -ne 0 ]; then
	BINPATH=$1
	echo "building to $BINPATH/webhook.arm"
fi
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o $BINPATH/webhook.arm
