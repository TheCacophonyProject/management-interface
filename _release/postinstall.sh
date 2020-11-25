#!/bin/bash
systemctl daemon-reload
systemctl enable managementd.service
systemctl restart managementd.service