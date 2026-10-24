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

import React from "react";
import {Button, Card, Col, InputNumber, Row} from "antd";
import * as SettingBackend from "./backend/SettingBackend";
import * as Setting from "./Setting";
import i18next from "i18next";

class SettingEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      classes: props,
      setting: null,
    };
  }

  UNSAFE_componentWillMount() {
    this.getSetting();
  }

  getSetting() {
    SettingBackend.getSetting()
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            setting: res.data,
          });
        } else {
          Setting.showMessage("error", `Failed to get setting: ${res.msg}`);
        }
      });
  }

  updateSettingField(key, value) {
    const setting = this.state.setting;
    setting[key] = value;
    this.setState({
      setting: setting,
    });
  }

  submitSettingEdit() {
    const setting = Setting.deepCopy(this.state.setting);
    SettingBackend.updateSetting(setting)
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to save: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Successfully saved");
          this.getSetting();
        }
      })
      .catch(error => {
        Setting.showMessage("error", `failed to save: ${error}`);
      });
  }

  renderSetting() {
    return (
      <Card size="small" title={
        <div>
          {i18next.t("general:Settings")}&nbsp;&nbsp;&nbsp;&nbsp;
          <Button type="primary" onClick={this.submitSettingEdit.bind(this)}>{i18next.t("general:Save")}</Button>
        </div>
      } style={{marginLeft: "5px"}} type="inner">
        <Row style={{marginTop: "10px"}} >
          <Col style={{marginTop: "5px"}} span={4}>
            {i18next.t("setting:Record retention days")}:
          </Col>
          <Col span={20} >
            <InputNumber min={0} value={this.state.setting.recordRetentionDays} onChange={value => {
              this.updateSettingField("recordRetentionDays", value ?? 0);
            }} />
            <div style={{marginTop: "5px", color: "gray"}}>
              {i18next.t("setting:Record retention days - Tooltip")}
            </div>
          </Col>
        </Row>
      </Card>
    );
  }

  render() {
    return (
      <div>
        <Row style={{width: "100%"}}>
          <Col span={1}>
          </Col>
          <Col span={22}>
            {
              this.state.setting !== null ? this.renderSetting() : null
            }
          </Col>
          <Col span={1}>
          </Col>
        </Row>
        <Row style={{margin: 10}}>
          <Col span={2}>
          </Col>
          <Col span={18}>
            <Button type="primary" size="large" onClick={this.submitSettingEdit.bind(this)}>{i18next.t("general:Save")}</Button>
          </Col>
        </Row>
      </div>
    );
  }
}

export default SettingEditPage;
