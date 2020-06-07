#!/bin/bash

if [ $UID != 0 ]; then
  echo "Installation must be run as root"
  exit 1
fi

RELEASE_FILE=$1

# Stop existing service
systemctl stop wossamessa.service

# User setup
mkdir -p /opt/wossamessa/data
adduser --system wossamessa
adduser wossamessa video
chown -R wossamessa /opt/wossamessa/data

# Files
RELEASE_FILE=`pwd`/$RELEASE_FILE
cd /opt/wossamessa
tar xzf $RELEASE_FILE --strip-components=1

# Service setup
ln -sf `pwd`/wossamessa.service /etc/systemd/system
systemctl daemon-reload
systemctl enable wossamessa.service
systemctl start wossamessa.service
