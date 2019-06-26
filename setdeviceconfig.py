#!/usr/bin/env python3

import os
import re
import yaml
from shutil import move


def split_yaml_params(yaml_raw, params):
    """ 
        returns yaml without specified params and yaml from params
        keeping comments that are above parameters
    """

    comment_chunk = ""
    clean_yaml = ""
    removed_yaml = ""
    param_regex = re.compile("^\\s*(\\S*):")
    comment_regex = re.compile("^\\s*#")
    empty_line = re.compile("^\\s*\\n")
    for line in yaml_raw.splitlines(True):
        if comment_regex.match(line):
            comment_chunk += line
        elif empty_line.match(line):
            clean_yaml += comment_chunk
            comment_chunk = ""
        else:
            param_match = param_regex.match(line)
            if param_match and param_match.group(1) in params:
                removed_yaml += comment_chunk + line
                comment_chunk = ""
            else:
                clean_yaml += comment_chunk + line
                comment_chunk = ""

    return clean_yaml, removed_yaml


def main():
    device_config = "/etc/cacophony/device.yaml"
    device_priv_config = "/etc/cacophony/device-priv.yaml"

    config = "/etc/thermal-uploader.yaml"
    private_config = "/etc/thermal-uploader-priv.yaml"
    device_params = ["server-url", "group", "device-name"]

    if os.path.isfile(device_config):
        print("{} already exists".format(device_config))
        exit()

    if not os.path.isfile(config):
        print("{}} does not exist".format(config))
        exit()

    with open(config, "r+") as f:
        config_contents = f.read()

    clean_yaml, device_yaml = split_yaml_params(config_contents, device_params)
    with open(device_config, "w+") as f:
        f.write(device_yaml)

    with open(config, "w") as f:
        f.write(clean_yaml)

    if os.path.isfile(private_config):
        move(private_config, device_priv_config)

if __name__ == "__main__":
    main()
