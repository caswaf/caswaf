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
import {Link} from "react-router-dom";
import {Button, Popconfirm, Switch, Table} from "antd";
import moment from "moment";
import * as Setting from "./Setting";
import * as NodeBackend from "./backend/NodeBackend";
import i18next from "i18next";
import BaseListPage from "./BaseListPage";

class NodeListPage extends BaseListPage {
  constructor(props) {
    super(props);
  }

  UNSAFE_componentWillMount() {
    this.setState({
      pagination: {
        ...this.state.pagination,
        current: 1,
        pageSize: 1000,
      },
    });
    this.fetch({pagination: this.state.pagination});
  }

  newNode() {
    const randomName = Setting.getRandomName();
    return {
      owner: this.props.account.name,
      name: `node_${randomName}`,
      createdTime: moment().format(),
      displayName: `New Node - ${randomName}`,
      tag: "",
      clientIp: "",
      upgradeMode: "At Any Time",
    };
  }

  addNode() {
    const newNode = this.newNode();
    NodeBackend.addNode(newNode)
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to add: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Node added successfully");
          this.setState({
            data: Setting.prependRow(this.state.data, newNode),
          });
          this.fetch();
        }
      }
      )
      .catch(error => {
        Setting.showMessage("error", `Node failed to add: ${error}`);
      });
  }

  deleteNode(i) {
    NodeBackend.deleteNode(this.state.data[i])
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to delete: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Node deleted successfully");
          this.fetch({
            pagination: {
              ...this.state.pagination,
              current: this.state.pagination.current > 1 && this.state.data.length === 1 ? this.state.pagination.current - 1 : this.state.pagination.current,
            },
          });
        }
      }
      )
      .catch(error => {
        Setting.showMessage("error", `Node failed to delete: ${error}`);
      });
  }

  renderTable(data) {
    const columns = [
      {
        title: i18next.t("general:Owner"),
        dataIndex: "owner",
        key: "owner",
        width: "90px",
        sorter: (a, b) => a.owner.localeCompare(b.owner),
      },
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "150px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        render: (text, record, index) => {
          return (
            <Link to={`/nodes/${record.owner}/${record.name}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("general:Create time"),
        dataIndex: "createdTime",
        key: "createdTime",
        width: "180px",
        sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      {
        title: i18next.t("general:Display name"),
        dataIndex: "displayName",
        key: "displayName",
        // width: "200px",
        sorter: (a, b) => a.displayName.localeCompare(b.displayName),
      },
      {
        title: i18next.t("general:Tag"),
        dataIndex: "tag",
        key: "tag",
        width: "140px",
        sorter: (a, b) => a.tag.localeCompare(b.tag),
      },
      {
        title: i18next.t("general:Client IP"),
        dataIndex: "clientIp",
        key: "clientIp",
        width: "150px",
        sorter: (a, b) => a.clientIp.localeCompare(b.clientIp),
      },
      {
        title: i18next.t("general:Upgrade mode"),
        dataIndex: "upgradeMode",
        key: "upgradeMode",
        width: "150px",
        sorter: (a, b) => a.upgradeMode.localeCompare(b.upgradeMode),
      },
      {
        title: i18next.t("general:Run mode"),
        dataIndex: "runMode",
        key: "runMode",
        width: "120px",
        sorter: (a, b) => (a.runMode || "").localeCompare(b.runMode || ""),
        render: (text, record, index) => {
          return text === "Binary" ? i18next.t("general:Binary") : i18next.t("general:Go Run");
        },
      },
      {
        title: i18next.t("general:Self update"),
        dataIndex: "selfUpdate",
        key: "selfUpdate",
        width: "120px",
        sorter: (a, b) => Number(a.selfUpdate) - Number(b.selfUpdate),
        render: (text, record, index) => {
          return <Switch disabled checkedChildren="ON" unCheckedChildren="OFF" checked={text} />;
        },
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "action",
        key: "action",
        width: "200px",
        fixed: (Setting.isMobile()) ? "false" : "right",
        render: (text, record, index) => {
          return (
            <div>
              <Button style={{marginTop: "10px", marginBottom: "10px", marginRight: "10px"}} type="primary" onClick={() => this.props.history.push(`/nodes/${record.owner}/${record.name}`)}>{i18next.t("general:Edit")}</Button>
              <Popconfirm
                title={`Sure to delete node: ${record.name} ?`}
                onConfirm={() => this.deleteNode(index)}
              >
                <Button style={{marginBottom: "10px"}} type="danger">{i18next.t("general:Delete")}</Button>
              </Popconfirm>
            </div>
          );
        },
      },
    ];

    const paginationProps = {
      pageSize: this.state.pagination.pageSize,
      total: this.state.pagination.total,
      showQuickJumper: true,
      showSizeChanger: true,
      showTotal: () => i18next.t("general:{total} in total").replace("{total}", this.state.pagination.total),
    };

    return (
      <div>
        <Table scroll={{x: "max-content"}} columns={columns} dataSource={data} rowKey={(record) => `${record.owner}/${record.name}`} size="middle" bordered pagination={paginationProps}
          title={() => (
            <div>
              {i18next.t("general:Nodes")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={this.addNode.bind(this)}>{i18next.t("general:Add")}</Button>
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }

  fetch = (params = {}) => {
    const field = params.searchedColumn, value = params.searchText;
    const sortField = params.sortField, sortOrder = params.sortOrder;
    if (params.pagination === undefined) {
      params.pagination = this.state.pagination;
    }
    this.setState({loading: true});
    // NodeBackend.getNodes(this.props.account.name, params.pagination.current, params.pagination.pageSize, field, value, sortField, sortOrder)
    NodeBackend.getNodes(this.props.account.name, "", "", field, value, sortField, sortOrder)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            loading: false,
            data: res.data,
            pagination: {
              ...params.pagination,
              total: res.data2,
            },
            searchedColumn: params.searchedColumn,
            searchText: params.searchText,
          });
        } else {
          if (Setting.isResponseDenied(res)) {
            this.setState({
              loading: false,
              isAuthorized: false,
            });
          } else {
            Setting.showMessage("error", res.msg);
          }
        }
      });
  };

  render() {
    return (
      <div>
        {
          this.renderTable(this.state.data)
        }
      </div>
    );
  }
}

export default NodeListPage;
