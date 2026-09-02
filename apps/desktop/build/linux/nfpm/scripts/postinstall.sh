#!/bin/sh
# SPDX-License-Identifier: MPL-2.0


if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications
fi

exit 0
