#!/bin/bash

if [ $UID != 0 ]; then
  echo "Installation must be run as root"
  exit 1
fi

RELEASE_FILE=$1

# Stop existing service
systemctl stop wossa.service

# User setup
mkdir -p /opt/wossa/data
adduser --system wossa
adduser wossa video
chown -R wossa /opt/wossa/data

# Files
RELEASE_FILE=`pwd`/$RELEASE_FILE
cd /opt/wossa
tar xzf $RELEASE_FILE --strip-components=1

# Service setup
ln -sf `pwd`/wossa.service /etc/systemd/system
systemctl daemon-reload
systemctl enable wossa.service
systemctl start wossa.service
