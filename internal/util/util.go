// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package util

import (
	"os/user"
)

func IsRoot() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}

	return u.Username == "root", nil
}
