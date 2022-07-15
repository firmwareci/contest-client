#!/bin/bash

COREBOOT_BINARY="build/coreboot.rom"
TIMEOUT="200"
LOGFILE="boot.log"

if [ ! -z $1 ]; then
	COREBOOT_BINARY="$1"
fi

if [ ! -z $2 ]; then
	IMAGE="$2"
fi

if [ ! -z $3 ]; then
	LOGFILE="$3"
fi

touch $LOGFILE

qemu-system-x86_64 -bios $COREBOOT_BINARY -m 8000 -smp 3 $IMAGE  -nographic -serial pipe:$LOGFILE &

PID=$!
sleep $TIMEOUT
kill $PID 

