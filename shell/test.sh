#!/bin/bash

VALUE=$(($RANDOM%89+10))

[ -f $HOME/wehook.txt ] || touch $HOME/wehook.txt

echo $VALUE > $HOME/wehook.txt

[ -f ./wehook.txt ] || touch ./wehook.txt

echo $VALUE > ./wehook.txt