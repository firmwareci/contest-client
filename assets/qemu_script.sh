#!/bin/bash

COREBOOT_BINARY="build/coreboot.rom"
TIMEOUT="30"

if [ ! -z $1 ]; then
	COREBOOT_BINARY="$1"
fi

if [ ! -z $2 ]; then
	TIMEOUT="$2"
fi

touch coreboot_qemu.log
qemu-system-x86_64 -bios $COREBOOT_BINARY -nographic -serial pipe:coreboot_qemu.log &
PID=$!
sleep $TIMEOUT
kill $PID 
cat coreboot_qemu.log
rm coreboot_qemu.log
