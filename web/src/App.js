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

import React, {Component} from "react";
import {Link, Redirect, Route, Switch, withRouter} from "react-router-dom";
import {Avatar, BackTop, Drawer, Dropdown, Layout, Menu, Tooltip} from "antd";
import {DeploymentUnitOutlined, DownOutlined, GithubOutlined, LogoutOutlined, SettingOutlined, ShareAltOutlined} from "@ant-design/icons";
import "./App.less";
import * as Setting from "./Setting";
import * as AccountBackend from "./backend/AccountBackend";
import AuthCallback from "./AuthCallback";
import * as Conf from "./Conf";
import HomePage from "./HomePage";
import NodeListPage from "./NodeListPage";
import NodeEditPage from "./NodeEditPage";
import SiteListPage from "./SiteListPage";
import SiteEditPage from "./SiteEditPage";
import CertListPage from "./CertListPage";
import CertEditPage from "./CertEditPage";
import RuleListPage from "./RuleListPage";
import RuleEditPage from "./RuleEditPage";
import SigninPage from "./SigninPage";
import RecordListPage from "./RecordListPage";
import RecordEditPage from "./RecordEditPage";
import i18next from "i18next";
import DashboardPage from "./DashboardPage";
import SettingEditPage from "./SettingEditPage";
import LanguageSelect from "./LanguageSelect";
import {withTranslation} from "react-i18next";
// import SelectLanguageBox from "./SelectLanguageBox";

const {Header, Footer} = Layout;

class App extends Component {
  constructor(props) {
    super(props);
    this.state = {
      classes: props,
      selectedMenuKey: 0,
      account: undefined,
      uri: null,
      isAiAssistantOpen: false,
    };

    Setting.initServerUrl();
    Setting.initCasdoorSdk(Conf.AuthConfig);
  }

  UNSAFE_componentWillMount() {
    this.updateMenuKey();
    this.getAccount();
  }

  componentDidUpdate() {
    // eslint-disable-next-line no-restricted-globals
    const uri = location.pathname;
    if (this.state.uri !== uri) {
      this.updateMenuKey();
    }
  }

  updateMenuKey() {
    // eslint-disable-next-line no-restricted-globals
    const uri = location.pathname;
    this.setState({
      uri: uri,
    });
    if (uri === "/") {
      this.setState({selectedMenuKey: "/"});
    } else if (uri.includes("/dashboard")) {
      this.setState({selectedMenuKey: "/dashboard"});
    } else if (uri.includes("/nodes")) {
      this.setState({selectedMenuKey: "/nodes"});
    } else if (uri.includes("/sites")) {
      this.setState({selectedMenuKey: "/sites"});
    } else if (uri.includes("/certs")) {
      this.setState({selectedMenuKey: "/certs"});
    } else if (uri.includes("/records")) {
      this.setState({selectedMenuKey: "/records"});
    } else if (uri.includes("/rules")) {
      this.setState({selectedMenuKey: "/rules"});
    } else if (uri.includes("/setting")) {
      this.setState({selectedMenuKey: "/setting"});
    } else {
      this.setState({selectedMenuKey: "null"});
    }
  }

  onUpdateAccount(account) {
    this.setState({
      account: account,
    });
  }

  setLanguage(account) {
    // let language = account?.language;
    const language = localStorage.getItem("language");
    if (language !== "" && language !== i18next.language) {
      Setting.setLanguage(language);
    }
  }

  openAiAssistant = () => {
    this.setState({
      isAiAssistantOpen: true,
    });
  };

  getAccount() {
    AccountBackend.getAccount()
      .then((res) => {
        const account = res.data;
        if (account !== null) {
          this.setLanguage(account);
          account.hostname = res.data2;
        }

        this.setState({
          account: account,
        });
      });
  }

  signout() {
    AccountBackend.signout()
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            account: null,
          });

          Setting.showMessage("success", "Successfully signed out, redirected to homepage");
          Setting.goToLink("/");
          // this.props.history.push("/");
        } else {
          Setting.showMessage("error", `Signout failed: ${res.msg}`);
        }
      });
  }

  handleRightDropdownClick(e) {
    if (e.key === "/account") {
      Setting.openLink(Setting.getMyProfileUrl(this.state.account));
    } else if (e.key === "/logout") {
      this.signout();
    }
  }

  renderAvatar() {
    if (this.state.account.avatar === "") {
      return (
        <Avatar style={{backgroundColor: Setting.getAvatarColor(this.state.account.name), verticalAlign: "middle"}} size="large">
          {Setting.getShortName(this.state.account.name)}
        </Avatar>
      );
    } else {
      return (
        <Avatar src={this.state.account.avatar} style={{verticalAlign: "middle"}} size="large">
          {Setting.getShortName(this.state.account.name)}
        </Avatar>
      );
    }
  }

  renderRightDropdown() {
    const menu = (
      <Menu onClick={this.handleRightDropdownClick.bind(this)}>
        <Menu.Item key="/account" icon={<SettingOutlined />}>
          {i18next.t("account:My Account")}
        </Menu.Item>
        <Menu.Item key="/logout" icon={<LogoutOutlined />}>
          {i18next.t("account:Sign Out")}
        </Menu.Item>
      </Menu>
    );

    return (
      <Dropdown key="/rightDropDown" overlay={menu} className="rightDropDown">
        <div className="ant-dropdown-link" style={{float: "right", cursor: "pointer"}}>
          &nbsp;
          &nbsp;
          {
            this.renderAvatar()
          }
          &nbsp;
          &nbsp;
          {Setting.isMobile() ? null : Setting.getShortName(this.state.account.displayName)} &nbsp; <DownOutlined />
          &nbsp;
          &nbsp;
          &nbsp;
        </div>
      </Dropdown>
    );
  }

  renderAccount() {
    const res = [];

    if (this.state.account === undefined) {
      return null;
    } else if (this.state.account === null) {
      res.push(
        <Menu.Item key="/signup" style={{float: "right", marginRight: "20px"}}>
          <a href={Setting.getSignupUrl()}>
            {i18next.t("account:Sign Up")}
          </a>
        </Menu.Item>
      );
      res.push(
        <Menu.Item key="/signin" style={{float: "right"}}>
          <a href={Setting.getSigninUrl()}>
            {i18next.t("account:Sign In")}
          </a>
        </Menu.Item>
      );
      res.push(
        <Menu.Item key="/" style={{float: "right"}}>
          <a href="/">
            {i18next.t("general:Home")}
          </a>
        </Menu.Item>
      );
      return (
        <Menu>
          {
            res
          }
        </Menu>
      );
    } else {
      res.push(
        <div style={{float: "right", display: "flex", alignItems: "center"}}>
          <Tooltip title="Click to open AI assitant" style={{marginRight: "50px", marginTop: "10px"}} >
            <div className="select-box" onClick={this.openAiAssistant}>
              <DeploymentUnitOutlined style={{fontSize: "24px"}} />
            </div>
          </Tooltip>
          <LanguageSelect />
          {this.renderRightDropdown()}
        </div>
      );
      return (
        <div style={{margin: "0px", padding: "0px"}}>
          {
            res
          }
        </div>
      );
    }
  }

  renderMenu() {
    const res = [];

    if (this.state.account === null || this.state.account === undefined) {
      return [];
    }

    res.push(
      <Menu.Item key="/">
        <a href="/">
          {i18next.t("general:Home")}
        </a>
        {/* <Link to="/">*/}
        {/*  Home*/}
        {/* </Link>*/}
      </Menu.Item>
    );

    res.push(
      <Menu.Item key="/dashboard">
        <Link to="/dashboard">
          {i18next.t("general:Dashboard")}
        </Link>
      </Menu.Item>
    );
    res.push(
      <Menu.Item key="/nodes">
        <Link to="/nodes">
          {i18next.t("general:Nodes")}
        </Link>
      </Menu.Item>
    );
    res.push(
      <Menu.Item key="/sites">
        <Link to="/sites">
          {i18next.t("general:Sites")}
        </Link>
      </Menu.Item>
    );
    res.push(
      <Menu.Item key="/certs">
        <Link to="/certs">
          {i18next.t("general:Certs")}
        </Link>
      </Menu.Item>
    );

    res.push(
      <Menu.Item key="/records">
        <Link to="/records">
          {i18next.t("general:Records")}
        </Link>
      </Menu.Item>
    );

    res.push(
      <Menu.Item key="/rules">
        <Link to="/rules">
          {i18next.t("general:Rules")}
        </Link>
      </Menu.Item>
    );

    res.push(
      <Menu.Item key="/setting">
        <Link to="/setting">
          {i18next.t("general:Settings")}
        </Link>
      </Menu.Item>
    );
    return res;
  }

  renderHomeIfSignedIn(component) {
    if (this.state.account !== null && this.state.account !== undefined) {
      return <Redirect to="/" />;
    } else {
      return component;
    }
  }

  renderSigninIfNotSignedIn(component) {
    if (this.state.account === null) {
      sessionStorage.setItem("from", window.location.pathname);
      return <Redirect to="/signin" />;
    } else if (this.state.account === undefined) {
      return null;
    } else {
      return component;
    }
  }

  renderContent() {
    return (
      <div>
        <Header style={{padding: "0", marginBottom: "3px", backgroundColor: "white", width: "100%"}} mode={"horizontal"}>
          {
            Setting.isMobile() ? null : (
              <Link to={"/"}>
                <div className="logo" />
              </Link>
            )
          }
          <Menu
            // theme="dark"
            mode={"horizontal"}
            selectedKeys={[`${this.state.selectedMenuKey}`]}
            style={{lineHeight: "64px", position: "absolute", left: 138, right: "300px"}}
          >
            {
              this.renderMenu()
            }
            {/* <SelectLanguageBox /> */}
          </Menu>
          {
            this.renderAccount()
          }
        </Header>
        <Switch>
          <Route exact path="/callback" component={AuthCallback} />
          <Route exact path="/home" render={(props) => <HomePage account={this.state.account} {...props} />} />
          <Route exact path="/" render={(props) => <Redirect to="/sites" />} />
          <Route exact path="/signin" render={(props) => this.renderHomeIfSignedIn(<SigninPage {...props} />)} />
          <Route exact path="/nodes" render={(props) => this.renderSigninIfNotSignedIn(<NodeListPage account={this.state.account} {...props} />)} />
          <Route exact path="/nodes/:owner/:nodeName" render={(props) => this.renderSigninIfNotSignedIn(<NodeEditPage account={this.state.account} {...props} />)} />
          <Route exact path="/sites" render={(props) => this.renderSigninIfNotSignedIn(<SiteListPage account={this.state.account} {...props} />)} />
          <Route exact path="/sites/:owner/:siteName" render={(props) => this.renderSigninIfNotSignedIn(<SiteEditPage account={this.state.account} {...props} />)} />
          <Route exact path="/certs" render={(props) => this.renderSigninIfNotSignedIn(<CertListPage account={this.state.account} {...props} />)} />
          <Route exact path="/certs/:owner/:certName" render={(props) => this.renderSigninIfNotSignedIn(<CertEditPage account={this.state.account} {...props} />)} />

          <Route exact path="/records" render={(props) => this.renderSigninIfNotSignedIn(<RecordListPage account={this.state.account} {...props} />)} />
          <Route exact path="/records/:owner/:id" render={(props) => this.renderSigninIfNotSignedIn(<RecordEditPage account={this.state.account} {...props} />)} />
          <Route exact path="/rules" render={(props) => this.renderSigninIfNotSignedIn(<RuleListPage account={this.state.account} {...props} />)} />
          <Route exact path="/rules/:owner/:ruleName" render={(props) => this.renderSigninIfNotSignedIn(<RuleEditPage account={this.state.account} {...props} />)} />
          <Route exact path="/setting" render={(props) => this.renderSigninIfNotSignedIn(<SettingEditPage account={this.state.account} {...props} />)} />
          <Route exact path="/dashboard" render={(props) => this.renderSigninIfNotSignedIn(<DashboardPage account={this.state.account} {...props} />)} />
        </Switch>
      </div>
    );
  }

  renderFooter() {
    // How to keep your footer where it belongs ?
    // https://www.freecodecamp.org/news/how-to-keep-your-footer-where-it-belongs-59c6aa05c59c/

    return (
      <Footer id="footer" style={
        {
          borderTop: "1px solid #e8e8e8",
          backgroundColor: "white",
          textAlign: "center",
        }
      }>
        Powered by <a target="_blank" href="https://github.com/casbin/caswaf" rel="noreferrer"><img style={{paddingBottom: "3px"}} height={"20px"} alt={"Casdoor"} src={`${Setting.StaticBaseUrl}/img/casbin_logo_1024x256.png`} /></a>
      </Footer>
    );
  }

  renderAiAssistant() {
    return (
      <Drawer
        title={
          <React.Fragment>
            <Tooltip title="Want to deploy your own AI assistant? Click to learn more!">
              <a target="_blank" rel="noreferrer" href={"https://casdoor.com"}>
                <img style={{width: "20px", marginRight: "10px", marginBottom: "2px"}} alt="help" src="https://casbin.org/img/casbin.svg" />
                AI Assistant
              </a>
            </Tooltip>
            <a className="custom-link" style={{float: "right", marginRight: "35px", marginTop: "2px"}} target="_blank" rel="noreferrer" href={"https://ai.casbin.com"}>
              <ShareAltOutlined className="custom-link" style={{fontSize: "20px", color: "rgb(140,140,140)"}} />
            </a>
            <a className="custom-link" style={{float: "right", marginRight: "30px", marginTop: "2px"}} target="_blank" rel="noreferrer" href={"https://github.com/casibase/casibase"}>
              <GithubOutlined className="custom-link" style={{fontSize: "20px", color: "rgb(140,140,140)"}} />
            </a>
          </React.Fragment>
        }
        placement="right"
        width={500}
        mask={false}
        onClose={() => {
          this.setState({
            isAiAssistantOpen: false,
          });
        }}
        visible={this.state.isAiAssistantOpen}
      >
        <iframe id="iframeHelper" title={"iframeHelper"} src={"https://ai.casbin.com/?isRaw=1"} width="100%" height="100%" scrolling="no" frameBorder="no" />
      </Drawer>
    );
  }

  render() {
    return (
      <div id="parent-area">
        <BackTop />
        <div id="content-wrap">
          {
            this.renderContent()
          }
        </div>
        {
          this.renderFooter()
        }
        {
          this.renderAiAssistant()
        }
      </div>
    );
  }
}

export default withRouter(withTranslation()(App));
