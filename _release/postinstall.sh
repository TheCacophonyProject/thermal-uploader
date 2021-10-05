#!/bin/bash
systemctl daemon-reload
systemctl enable thermal-uploader.service
systemctl restart thermal-uploader.service
