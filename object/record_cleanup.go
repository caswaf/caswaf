// Copyright 2026 The casbin Authors. All Rights Reserved.
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

package object

import (
	"fmt"
	"time"

	"github.com/casbin/caswaf/util"
)

const (
	recordCleanupBatchSize = 1000
	// spreads the deletes over time, so that the binlog they generate does not fill up the database's disk at once
	recordCleanupBatchInterval = 100 * time.Millisecond
)

// cleanupRecords deletes the records created before the retention days in the global setting. The records are
// scanned in the order of their ids, because "created_time" has no index and the ids grow with it.
func cleanupRecords() error {
	setting, err := GetSetting()
	if err != nil {
		return err
	}
	if setting.RecordRetentionDays <= 0 {
		return nil
	}

	// formatted in the same time zone as util.GetCurrentTime(), so that the strings compare in time order
	location, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		return err
	}
	cutoffTime := time.Now().AddDate(0, 0, -setting.RecordRetentionDays).In(location).Format(time.RFC3339)
	deletedCount := int64(0)
	for {
		records := []*Record{}
		err = ormer.Engine.Cols("id", "created_time").Asc("id").Limit(recordCleanupBatchSize).Find(&records)
		if err != nil {
			return err
		}

		ids := []int64{}
		for _, record := range records {
			if record.CreatedTime >= cutoffTime {
				break
			}
			ids = append(ids, record.Id)
		}
		if len(ids) == 0 {
			break
		}

		affected, err := ormer.Engine.In("id", ids).Delete(&Record{})
		if err != nil {
			return err
		}
		deletedCount += affected

		if len(ids) < len(records) {
			break
		}
		time.Sleep(recordCleanupBatchInterval)
	}

	if deletedCount != 0 {
		fmt.Printf("[%s] Deleted [%d] records created before: %s\n", util.GetCurrentTime(), deletedCount, cutoffTime)
	}
	return nil
}

func StartRecordCleanupLoop() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[%s] Recovered from StartRecordCleanupLoop() panic: %v\n", util.GetCurrentTime(), r)
				StartRecordCleanupLoop()
			}
		}()

		for {
			err := cleanupRecords()
			if err != nil {
				fmt.Printf("[%s] cleanupRecords() error: %s\n", util.GetCurrentTime(), err.Error())
			}

			time.Sleep(time.Hour)
		}
	}()
}
