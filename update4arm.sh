#!/bin/bash

TAG=$(<VERSION)
echo $TAG

./build4arm.sh .

docker build -t 192.168.5.46:5000/armhf-webhook:$TAG .
docker push 192.168.5.46:5000/armhf-webhook:$TAG

echo "clearing ..."
rm *.arm
