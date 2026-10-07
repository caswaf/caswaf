// Copyright 2025 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package run

import (
	"fmt"
	"strings"

	"github.com/beego/beego"
	"github.com/xorm-io/xorm"
)

// DropDatabase drops the database of a Casdoor customer site in the local MySQL, the one gitCreateDatabase() created.
// It refuses any other site, and nodes whose databases are in RDS.
func DropDatabase(siteName string) error {
	if !strings.HasPrefix(siteName, "casdoor_customer_") || getNameIndex(siteName) == 0 || beego.AppConfig.String("dbInstanceId") != "" {
		return fmt.Errorf("DropDatabase() error, refusing to drop the database of site %s", siteName)
	}
	name := strings.Replace(siteName, "_00", "_", 1)
	fmt.Printf("DropDatabase(): [%s]\n", name)

	dataSourceName := fmt.Sprintf("root:%s@tcp(localhost:3306)/", beego.AppConfig.String("dbPass"))
	engine, err := xorm.NewEngine("mysql", dataSourceName)
	if err != nil {
		return err
	}
	defer engine.Close()

	_, err = engine.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	return err
}

func gitCreateDatabase(name string) (bool, error) {
	fmt.Printf("gitCreateDatabase(): [%s]\n", name)
	name = strings.Replace(name, "_00", "_", 1)

	driverName := "mysql"
	dataSourceName := fmt.Sprintf("root:%s@tcp(localhost:3306)/", beego.AppConfig.String("dbPass"))
	engine, err := xorm.NewEngine(driverName, dataSourceName)
	if err != nil {
		return false, err
	}

	cmd := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", name)
	result, err := engine.Exec(cmd)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	engine.Close()

	return affected != 0, err
}
