// Copyright 2023 The casbin Authors. All Rights Reserved.
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
import {Button, Popconfirm, Table} from "antd";
import moment from "moment";
import * as Setting from "./Setting";
import * as CertBackend from "./backend/CertBackend";
import i18next from "i18next";
import copy from "copy-to-clipboard";
import BaseListPage from "./BaseListPage";

class CertListPage extends BaseListPage {
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

  newCert() {
    const randomName = Setting.getRandomName();
    return {
      owner: this.props.account.name,
      name: `cert_${randomName}`,
      createdTime: moment().format(),
      displayName: `New Cert - ${randomName}`,
      type: "SSL",
      cryptoAlgorithm: "RSA",
      expireTime: "",
      certificate: "",
      privateKey: "",
    };
  }

  addCert() {
    const newCert = this.newCert();
    CertBackend.addCert(newCert)
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to add: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Cert added successfully");
          this.setState({
            data: Setting.prependRow(this.state.data, newCert),
          });
          this.fetch();
        }
      }
      )
      .catch(error => {
        Setting.showMessage("error", `Cert failed to add: ${error}`);
      });
  }

  deleteCert(i) {
    CertBackend.deleteCert(this.state.data[i])
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to delete: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Cert deleted successfully");
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
        Setting.showMessage("error", `Cert failed to delete: ${error}`);
      });
  }

  refreshCert(i) {
    const cert = this.state.data[i];
    CertBackend.refreshDomainExpire(cert.owner, cert.name)
      .then((res) => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to refresh domain expire: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Domain expire refresh successfully");
          this.fetch();
        }
      }
      )
      .catch(error => {
        Setting.showMessage("error", `Domain expire failed to refresh: ${error}`);
      });
  }

  async refreshAllCerts() {
    const certs = this.state.data;
    const failures = [];
    this.setState({refreshAllProgress: 0});
    for (let i = 0; i < certs.length; i++) {
      const cert = certs[i];
      try {
        const res = await CertBackend.refreshDomainExpire(cert.owner, cert.name);
        if (res.status === "error") {
          failures.push(`${cert.name}: ${res.msg}`);
        }
      } catch (error) {
        failures.push(`${cert.name}: ${error}`);
      }
      this.setState({refreshAllProgress: i + 1});
    }
    this.setState({refreshAllProgress: undefined});

    if (failures.length === 0) {
      Setting.showMessage("success", `All ${certs.length} certs refreshed successfully`);
    } else {
      Setting.showMessage("error", `${failures.length}/${certs.length} certs failed to refresh: ${failures.join("; ")}`);
    }
    this.fetch({pagination: this.state.pagination});
  }

  renderTable(data) {
    const columns = [
      {
        title: i18next.t("general:Owner"),
        dataIndex: "owner",
        key: "owner",
        width: "120px",
        sorter: (a, b) => a.owner.localeCompare(b.owner),
      },
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "120px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        render: (text, record, index) => {
          return (
            <Link to={`/certs/${record.owner}/${record.name}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("general:Create time"),
        dataIndex: "createdTime",
        key: "createdTime",
        width: "150px",
        sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      // {
      //   title: i18next.t("general:Display name"),
      //   dataIndex: "displayName",
      //   key: "displayName",
      //   // width: "200px",
      //   sorter: (a, b) => a.displayName.localeCompare(b.displayName),
      // },
      // {
      //   title: i18next.t("cert:Type"),
      //   dataIndex: "type",
      //   key: "type",
      //   width: "180px",
      //   sorter: (a, b) => a.type.localeCompare(b.type),
      // },
      // {
      //   title: i18next.t("cert:Crypto algorithm"),
      //   dataIndex: "cryptoAlgorithm",
      //   key: "cryptoAlgorithm",
      //   width: "180px",
      //   sorter: (a, b) => a.cryptoAlgorithm.localeCompare(b.cryptoAlgorithm),
      // },
      {
        title: i18next.t("cert:Expire time"),
        dataIndex: "expireTime",
        key: "expireTime",
        width: "150px",
        sorter: (a, b) => a.expireTime.localeCompare(b.expireTime),
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      {
        title: i18next.t("cert:Domain expire"),
        dataIndex: "domainExpireTime",
        key: "domainExpireTime",
        width: "150px",
        sorter: (a, b) => a.domainExpireTime.localeCompare(b.domainExpireTime),
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      {
        title: i18next.t("cert:Provider"),
        dataIndex: "provider",
        key: "provider",
        width: "110px",
        sorter: (a, b) => a.provider.localeCompare(b.provider),
      },
      {
        title: i18next.t("cert:Account"),
        dataIndex: "account",
        key: "account",
        width: "120px",
        sorter: (a, b) => a.account.localeCompare(b.account),
      },
      // {
      //   title: i18next.t("cert:Access key"),
      //   dataIndex: "accessKey",
      //   key: "accessKey",
      //   width: "180px",
      //   sorter: (a, b) => a.accessKey.localeCompare(b.accessKey),
      // },
      // {
      //   title: i18next.t("cert:Access secret"),
      //   dataIndex: "accessSecret",
      //   key: "accessSecret",
      //   width: "180px",
      //   sorter: (a, b) => a.accessSecret.localeCompare(b.accessSecret),
      // },
      {
        title: i18next.t("cert:Certificate"),
        dataIndex: "certificate",
        key: "certificate",
        width: "180px",
        sorter: (a, b) => a.certificate.localeCompare(b.certificate),
        render: (text, record, index) => {
          return (
            <Button onClick={() => {
              copy(text);
              Setting.showMessage("success", i18next.t("cert:Certificate copied to clipboard successfully"));
            }}
            >
              {Setting.getShortText(text.replace(/^-----BEGIN CERTIFICATE-----/, ""), 15)}
            </Button>
          );
        },
      },
      {
        title: i18next.t("cert:Private key"),
        dataIndex: "privateKey",
        key: "privateKey",
        width: "180px",
        sorter: (a, b) => a.privateKey.localeCompare(b.privateKey),
        render: (text, record, index) => {
          return (
            <Button onClick={() => {
              copy(text);
              Setting.showMessage("success", i18next.t("cert:Private key copied to clipboard successfully"));
            }}
            >
              {Setting.getShortText(text.replace(/^-----BEGIN RSA PRIVATE KEY-----/, ""), 15)}
            </Button>
          );
        },
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "action",
        key: "action",
        width: "260px",
        render: (text, record, index) => {
          return (
            <div style={{display: "flex", alignItems: "center", flexWrap: "nowrap"}}>
              <Button style={{margin: "10px 10px 10px 0"}} type="default" onClick={() => this.refreshCert(index)}>{i18next.t("general:Refresh")}
              </Button>
              <Button style={{margin: "10px 10px 10px 0"}} type="primary" onClick={() => this.props.history.push(`/certs/${record.owner}/${record.name}`)}>{i18next.t("general:Edit")}</Button>
              <Popconfirm
                title={`Sure to delete cert: ${record.name} ?`}
                onConfirm={() => this.deleteCert(index)}
                okText="OK"
                cancelText="Cancel"
              >
                <Button style={{margin: "10px 10px 10px 0"}} type="danger">{i18next.t("general:Delete")}</Button>
              </Popconfirm>
            </div>
          );
        },
      },
    ];

    return (
      <div>
        <Table
          columns={columns} dataSource={data} rowKey="name" size="middle" bordered pagination={this.state.pagination}
          title={() => (
            <div>
              {i18next.t("general:Certs")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={this.addCert.bind(this)}>{i18next.t("general:Add")}</Button>
              &nbsp;&nbsp;
              <Button size="small" loading={this.state.refreshAllProgress !== undefined} disabled={!this.state.data || this.state.data.length === 0} onClick={this.refreshAllCerts.bind(this)}>
                {i18next.t("general:Refresh all")}
                {this.state.refreshAllProgress !== undefined ? ` (${this.state.refreshAllProgress}/${this.state.data.length})` : ""}
              </Button>
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
    if (!params.pagination) {
      params.pagination = {current: 1, pageSize: 100};
    }
    this.setState({loading: true});
    CertBackend.getCerts(this.props.account.name, params.pagination.current, params.pagination.pageSize, field, value, sortField, sortOrder)
      .then((res) => {
        this.setState({
          loading: false,
        });
        if (res.status === "ok") {
          this.setState({
            data: res.data,
            pagination: {
              ...params.pagination,
              total: res.data2,
            },
            searchText: params.searchText,
            searchedColumn: params.searchedColumn,
            sortField: sortField,
            sortOrder: sortOrder,
          });
        } else {
          Setting.showMessage("error", `Failed to get certs: ${res.msg}`);
        }
      });
  };

}

export default CertListPage;
