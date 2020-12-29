#!/bin/bash
if [ $UID != 0 ]; then
  echo "Installation must be run as root"
  exit 1
fi

# Stop existing service
systemctl stop wossa.service

# User setup
adduser --system wossa
adduser wossa video
mkdir -p /opt/wossa/data
chown -R wossa /opt/wossa/data

# Files
cp -r `dirname $0`/* /opt/wossa

# Service setup
ln -sf /opt/wossa/wossa.service /etc/systemd/system
systemctl daemon-reload
systemctl enable wossa.service
systemctl start wossa.service
