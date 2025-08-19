#!/bin/bash
systemctl daemon-reload
systemctl enable managementd.service

# Using start instead of restart to avoid restarting the service if it's already running, causing API issues during a salt update.
systemctl start managementd.service
