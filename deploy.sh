#!/usr/bin/env bash

set -e
cd $(dirname $0)

if [ "$#" -ne 3 ]; then
	echo "usage: $0 /home/user/.ssh/identity_file user@server-address /path/to/remote/directory/"
	exit 1
fi

IDENTITY_FILE=$1
SERVER_SSH=$2
SERVER_PATH=$3
BINARY_NAME="maze-racer"
SERVER_RESTART_COMMAND="systemctl --user restart $BINARY_NAME"

GOOS=linux GOARCH=amd64 go build -o maze-racer

OUTFILE="./$BINARY_NAME"
COMMIT_HASH=$(git rev-parse HEAD)
BUILD_TIMESTAMP=$(TZ=UTC date -u +"%s")
FILE_HASH=$(b2sum $OUTFILE | cut -f1 -d' ')
REMOTE_FILENAME="$BINARY_NAME-$BUILD_TIMESTAMP-$COMMIT_HASH-$FILE_HASH"

ssh -i $IDENTITY_FILE $SERVER_SSH "mkdir -p $SERVER_PATH/versions/"
scp -i $IDENTITY_FILE "$OUTFILE" "$SERVER_SSH:$SERVER_PATH/versions/$REMOTE_FILENAME"
ssh -i $IDENTITY_FILE -q -T $SERVER_SSH <<EOL
	nohup sh -c "\
	rm "$SERVER_PATH/$BINARY_NAME" && \
	ln -s "$SERVER_PATH/versions/$REMOTE_FILENAME" "$SERVER_PATH/$BINARY_NAME" && \
	$SERVER_RESTART_COMMAND"
EOL