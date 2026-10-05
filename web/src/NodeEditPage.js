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

import React from "react";
import {Button, Card, Col, Input, Row, Select} from "antd";
import * as NodeBackend from "./backend/NodeBackend";
import * as Setting from "./Setting";
import i18next from "i18next";

const {Option} = Select;

class NodeEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      classes: props,
      owner: props.match.params.owner,
      nodeName: props.match.params.nodeName,
      node: null,
    };
  }

  UNSAFE_componentWillMount() {
    this.getNode();
  }

  getNode() {
    NodeBackend.getNode(this.state.owner, this.state.nodeName)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            node: res.data,
          });
        } else {
          Setting.showMessage("error", `Failed to get node: ${res.msg}`);
        }
      });
  }

  parseNodeField(key, value) {
    return value;
  }

  updateNodeField(key, value) {
    value = this.parseNodeField(key, value);

    const node = this.state.node;
    node[key] = value;
    this.setState({
      node: node,
    });
  }

  renderNode() {
    return (
      <Card size="small" title={
        <div>
          {i18next.t("node:Edit Node")}&nbsp;&nbsp;&nbsp;&nbsp;
          <Button type="primary" onClick={this.submitNodeEdit.bind(this)}>{i18next.t("general:Save")}</Button>
        </div>
      } style={{marginLeft: "5px"}} type="inner">
        <Row style={{marginTop: "10px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Name")}:
          </Col>
          <Col span={22} >
            <Input value={this.state.node.name} onChange={e => {
              this.updateNodeField("name", e.target.value);
            }} />
          </Col>
        </Row>
        <Row style={{marginTop: "20px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Display name")}:
          </Col>
          <Col span={22} >
            <Input value={this.state.node.displayName} onChange={e => {
              this.updateNodeField("displayName", e.target.value);
            }} />
          </Col>
        </Row>
        <Row style={{marginTop: "20px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Tag")}:
          </Col>
          <Col span={22} >
            <Input value={this.state.node.tag} onChange={e => {
              this.updateNodeField("tag", e.target.value);
            }} />
          </Col>
        </Row>
        <Row style={{marginTop: "20px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Client IP")}:
          </Col>
          <Col span={22} >
            <Input value={this.state.node.clientIp} onChange={e => {
              this.updateNodeField("clientIp", e.target.value);
            }} />
          </Col>
        </Row>
        <Row style={{marginTop: "20px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Upgrade mode")}:
          </Col>
          <Col span={22} >
            <Select virtual={false} style={{width: "100%"}} value={this.state.node.upgradeMode} onChange={(value => {
              this.updateNodeField("upgradeMode", value);
            })}>
              <Option key="At Any Time" value="At Any Time">{i18next.t("general:At Any Time")}</Option>
              <Option key="No Upgrade" value="No Upgrade">{i18next.t("general:No Upgrade")}</Option>
              <Option key="Half A Hour" value="Half A Hour">{i18next.t("general:Half A Hour")}</Option>
            </Select>
          </Col>
        </Row>
        <Row style={{marginTop: "20px"}} >
          <Col style={{marginTop: "5px"}} span={2}>
            {i18next.t("general:Run mode")}:
          </Col>
          <Col span={22} >
            <Select virtual={false} style={{width: "100%"}} value={this.state.node.runMode || ""} onChange={(value => {
              this.updateNodeField("runMode", value);
            })}>
              <Option key="" value="">{i18next.t("general:Go Run")}</Option>
              <Option key="Binary" value="Binary">{i18next.t("general:Binary")}</Option>
            </Select>
          </Col>
        </Row>
      </Card>
    );
  }

  submitNodeEdit() {
    const node = Setting.deepCopy(this.state.node);
    NodeBackend.updateNode(this.state.node.owner, this.state.nodeName, node)
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to save: ${res.msg}`);
          this.updateNodeField("name", this.state.nodeName);
        } else {
          Setting.showMessage("success", "Successfully saved");
          this.setState({
            nodeName: this.state.node.name,
          });
          this.props.history.push(`/nodes/${this.state.node.owner}/${this.state.node.name}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `Failed to connect to server: ${error}`);
      });
  }

  render() {
    return (
      <div>
        {
          this.state.node !== null ? this.renderNode() : null
        }
        <div style={{marginTop: "20px", marginLeft: "40px"}}>
          <Button size="large" onClick={() => this.props.history.push("/nodes")}>{i18next.t("general:Cancel")}</Button>
        </div>
      </div>
    );
  }
}

export default NodeEditPage;
