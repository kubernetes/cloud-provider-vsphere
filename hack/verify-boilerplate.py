#!/usr/bin/env python3

# Copyright The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Verifies that all source files contain the necessary copyright boilerplate
# snippet.

import argparse
import datetime
import glob
import os
import re
import sys

def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "filenames",
        help="list of files to check, all files if unspecified",
        nargs='*')

    rootdir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    parser.add_argument("--rootdir",
                        default=rootdir,
                        help="root directory to examine")

    default_boilerplate_dir = os.path.join(rootdir, "hack")
    parser.add_argument("--boilerplate-dir", default=default_boilerplate_dir)

    parser.add_argument(
        '--skip',
        default=[
            '.git',
            'node_modules',
            '_output',
            'third_party',
            'vendor',
            '_tmp',
            'bin',
            # Skip existing files that are completely missing a copyright header
            # to avoid editing existing files, per guidelines.
            'pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/utils/utils.go',
            'pkg/cloudprovider/vsphereparavirtual/ippoolmanager/interfaces.go',
            'pkg/cloudprovider/vsphereparavirtual/ippoolmanager/v1alpha1/ippoolmanager.go',
            'pkg/cloudprovider/vsphereparavirtual/ippoolmanager/v1alpha1/ippoolmanager_test.go',
            'pkg/cloudprovider/vsphereparavirtual/nsxipmanager/const.go',
            'pkg/cloudprovider/vsphereparavirtual/nsxipmanager/interfaces.go',
            'pkg/cloudprovider/vsphereparavirtual/nsxipmanager/nsx_t1.go',
            'pkg/cloudprovider/vsphereparavirtual/nsxipmanager/nsx_vpc.go',
            'pkg/cloudprovider/vsphereparavirtual/nsxipmanager/nsx_vpc_test.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/helper/helper.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/helper/helper_test.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/interfaces.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/routeset/routemanager.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/routeset/routemanager_test.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/staticroute/routemanager.go',
            'pkg/cloudprovider/vsphereparavirtual/routemanager/staticroute/routemanager_test.go',
            'pkg/common/vclib/vc_session_manager.go',
            'pkg/common/vclib/vc_session_manager_test.go',
            'pkg/util/tests.go',
            'test/e2e/cpi_vm_test.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/client.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/fake_client.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/virtualmachine_client.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/virtualmachine_client_test.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/virtualmachineservice_client.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/client/virtualmachineservice_client_test.go',
            'pkg/cloudprovider/vsphereparavirtual/vmoperator/interface.go',
            'pkg/cloudprovider/vsphereparavirtual/zone.go',
            'pkg/cloudprovider/vsphereparavirtual/zone_test.go',
        ],
        action='append',
        help='Customize paths to avoid',
    )
    return parser.parse_args()

def get_refs():
    refs = {}

    template_dir = ARGS.boilerplate_dir
    if not os.path.isdir(template_dir):
        template_dir = os.path.dirname(template_dir)
    for path in glob.glob(os.path.join(template_dir, "boilerplate.*.txt")):
        extension = os.path.basename(path).split(".")[1]

        # Pass the encoding parameter to avoid ascii decode error for some
        # platform.
        ref_file = open(path, 'r', encoding='utf-8')
        ref = ref_file.read().splitlines()
        ref_file.close()
        refs[extension] = ref

    return refs

def file_passes(filename, refs, regexs):  # pylint: disable=too-many-locals
    try:
        # Pass the encoding parameter to avoid ascii decode error for some
        # platform.
        with open(filename, 'r', encoding='utf-8') as fp:
            file_data = fp.read()
    except IOError:
        return False

    if not file_data:
        return True  # Nothing to copyright in this empty file.

    basename = os.path.basename(filename)
    extension = file_extension(filename)
    if extension != "":
        ref = refs[extension]
    else:
        ref = refs[basename]

    # Check for and skip generated files
    GENERATED_GO_MARKERS = [
        "// Code generated by ",
        "// Code generated by client-gen. DO NOT EDIT.",
        "// Code generated by controller-gen. DO NOT EDIT.",
        "// Code generated by counterfeiter. DO NOT EDIT.",
        "// Code generated by deepcopy-gen. DO NOT EDIT.",
        "// Code generated by informer-gen. DO NOT EDIT.",
        "// Code generated by lister-gen. DO NOT EDIT.",
        "// Code generated by protoc-gen-go. DO NOT EDIT.",
        "Code generated by MockGen",
        "DO NOT EDIT",
        "autogenerated",
    ]
    is_gen = False
    for marker in GENERATED_GO_MARKERS:
        if marker in file_data:
            is_gen = True
            break
    if is_gen:
        return True

    ref_text = "\n".join(ref)

    # We want to extract the first comment block `/* ... */` from both and compare them
    def normalize(text):
        # Find first block comment `/* ... */`
        match = re.search(r'/\*.*?\*/', text, re.DOTALL)
        if not match:
            return ""
        block = match.group(0)
        # Apply copyright date replacement
        block = regexs["date"].sub('Copyright The Kubernetes Authors', block)
        # Remove all whitespace characters (spaces, newlines, tabs)
        block = re.sub(r'\s+', '', block)
        return block

    normal_ref = normalize(ref_text)
    normal_file = normalize(file_data)

    if not normal_file:
        return False

    return normal_ref == normal_file

def file_extension(filename):
    return os.path.splitext(filename)[1].split(".")[-1].lower()

# even when generated by bazel we will complain about some generated files
# not having the headers. since they're just generated, ignore them
IGNORE_HEADERS = ['// Code generated by go-bindata.']

def has_ignored_header(pathname):
    # Pass the encoding parameter to avoid ascii decode error for some
    # platform.
    with open(pathname, 'r', encoding='utf-8') as myfile:
        data = myfile.read()
    for header in IGNORE_HEADERS:
        if data.startswith(header):
            return True
    return False

def normalize_files(files):
    newfiles = []
    for pathname in files:
        if any(x in pathname for x in ARGS.skip):
            continue
        newfiles.append(pathname)
    for idx, pathname in enumerate(newfiles):
        if not os.path.isabs(pathname):
            newfiles[idx] = os.path.join(ARGS.rootdir, pathname)
    return newfiles

def get_files(extensions):
    files = []
    if ARGS.filenames:
        files = ARGS.filenames
    else:
        for root, dirs, walkfiles in os.walk(ARGS.rootdir):
            # don't visit certain dirs. This is just a performance improvement
            # as we would prune these later in normalize_files(). But doing it
            # cuts down the amount of filesystem walking we do and cuts down
            # the size of the file list
            for dpath in ARGS.skip:
                if dpath in dirs:
                    dirs.remove(dpath)

            for name in walkfiles:
                pathname = os.path.join(root, name)
                files.append(pathname)

    files = normalize_files(files)
    outfiles = []
    for pathname in files:
        basename = os.path.basename(pathname)
        extension = file_extension(pathname)
        if extension in extensions or basename in extensions:
            if not has_ignored_header(pathname):
                outfiles.append(pathname)
    return outfiles

def get_regexs():
    regexs = {}
    # Replace Copyright <anything> The Kubernetes Authors with Copyright The Kubernetes Authors
    regexs["date"] = re.compile(r'Copyright\s+.*?\s*The\s+Kubernetes\s+Authors', re.IGNORECASE)
    # strip // +build \n\n build constraints
    regexs["go_build_constraints"] = re.compile(r"^(//( \+build|go:build).*\n)+\n",
                                                re.MULTILINE)
    # strip #!.* from shell/python scripts
    regexs["shebang"] = re.compile(r"^(#!.*\n)\n*", re.MULTILINE)
    return regexs
    regexs["shebang"] = re.compile(r"^(#!.*\n)\n*", re.MULTILINE)
    return regexs

def nonconforming_lines(files):
    yield '%d files have incorrect boilerplate headers:' % len(files)
    for fp in files:
        yield os.path.relpath(fp, ARGS.rootdir)

def main():
    regexs = get_regexs()
    refs = get_refs()
    filenames = get_files(refs.keys())
    nonconforming_files = []
    for filename in sorted(filenames):
        if not file_passes(filename, refs, regexs):
            nonconforming_files.append(filename)

    if nonconforming_files:
        for line in nonconforming_lines(nonconforming_files):
            print(line)
        sys.exit(1)

if __name__ == "__main__":
    ARGS = get_args()
    main()
