#!/usr/bin/env python3

import os
import re
import yaml
from shutil import move


DEVICE_CONFIG = "/etc/cacophony/device.yaml"
DEVICE_PRIV_CONFIG = "/etc/cacophony/device-priv.yaml"
CONFIG = "/etc/thermal-uploader.yaml"
PRIVATE_CONFIG = "/etc/thermal-uploader-priv.yaml"
DEVICE_PARAMS = ["server-url", "group", "device-name"]


def split_yaml_params(yaml_raw, params):
    """ 
        returns yaml without specified params and yaml from params
        keeping comments that are above parameters
    """

    comment_chunk = ""
    clean_yaml = ""
    removed_yaml = ""
    param_regex = re.compile("^\\s*(\\S+):")
    comment_regex = re.compile("^\\s*#")
    for line in yaml_raw.splitlines(True):
        if comment_regex.match(line):
            comment_chunk += line
        elif line.strip() == "":
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
    if os.path.isfile(DEVICE_CONFIG):
        print("{} already exists".format(DEVICE_CONFIG))
        exit()

    if not os.path.isfile(CONFIG):
        print("{} does not exist".format(CONFIG))
        exit()

    with open(CONFIG, "r+") as f:
        config_contents = f.read()

    clean_yaml, device_yaml = split_yaml_params(config_contents, DEVICE_PARAMS)
    with open(DEVICE_CONFIG, "w+") as f:
        f.write(device_yaml)

    with open(CONFIG, "w") as f:
        f.write(clean_yaml)

    if os.path.isfile(PRIVATE_CONFIG):
        move(PRIVATE_CONFIG, DEVICE_PRIV_CONFIG)


if __name__ == "__main__":
    main()
