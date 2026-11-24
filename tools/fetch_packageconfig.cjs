"use strict";
const fs = require("node:fs");
const https = require("node:https");
const path = require("node:path");
const os = require("node:os");

const root = __dirname;
const native = require(path.join(root, "resources/app.asar.unpacked/node_modules/napi-cutils/build/Release/cutils.node"));
const client = new native.CUtils();
const config = JSON.parse(client.extApi1(path.join(root, "res/app_config.dat")));
if (!config.launcherId || !config.appId || !config.channelCode || !config.secretKey) {
  throw Error("Original app config is missing required fields");
}
client.extApi2(config.secretKey, config.appId, config.channelCode);
const bundle = fs.readFileSync(path.join(root, "resources/app.asar"));
const version = bundle.toString("latin1").match(/launcher@([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)/)?.[1];
if (!version) throw Error("Launcher version not found");
const data = {launcherId: String(config.launcherId), uid: ""};
const query = Object.keys(data).sort().map(k => k + "=" + data[k]).join("&");
const headers = {
  "grp-app-id": String(config.appId),
  "grp-platform": "Launcher",
  "grp-channel-code": String(config.channelCode),
  "grp-sub-channel-code": "SC000",
  "sdk-version": version,
  "channel-sdk-version": version,
  "grp-launcher-version-code": version,
  lang: "zh-Hans",
  "grp-client-info": JSON.stringify({device_model:"Launcher", device_os:os.type(), device_os_version:os.release(), app_version:version})
};
// Only the normal anonymous package request. Do not collect account tokens,
// export keys, disable TLS validation, or work around authorization failures.
function get(host) {
  return new Promise((resolve, reject) => {
    const timestamp = Date.now();
    const sign = client.extApi3(String(timestamp) + query);
    const req = https.get("https://" + host + "/pc-launcher/client/gameversion/packageconfig?" + new URLSearchParams(data), {
      headers: {...headers, timestamp: String(timestamp), sign: String(sign)}
    }, res => {
      let raw = "";
      res.setEncoding("utf8");
      res.on("data", s => {
        raw += s;
        if (raw.length > 8 * 1024 * 1024) req.destroy(Error("Response too large"));
      });
      res.on("error", reject);
      res.on("end", () => {
        if (res.statusCode !== 200) return reject(Error("HTTP " + res.statusCode));
        try {
          const parsed = JSON.parse(raw);
          if (parsed.code !== 0) throw Error("Official API code " + parsed.code);
          if (!parsed.data?.released?.gamePackageList?.length && !parsed.data?.preload?.gamePackageList?.length) throw Error("No game packages in response");
          resolve(parsed);
        } catch (e) { reject(e); }
      });
    });
    req.setTimeout(15000, () => req.destroy(Error("request timeout")));
    req.on("error", reject);
  });
}
(async () => {
  let last;
  for (const host of ["api-grp.manjuu.com","api-grp-l2.manjuu.com"]) {
    try {
      const response = await get(host);
      // Keep only fields needed by the downloader; no user/session information.
      const result = {code:0,data:{}};
      for (const name of ["released","preload"]) {
        const r = response.data[name];
        if (!r) continue;
        result.data[name] = {
          versionCode:r.versionCode, versionName:r.versionName, domainList:r.domainList,
          gamePackageList:(r.gamePackageList || []).map(p => ({
            packageType:p.packageType, downloadUrl:p.downloadUrl, fileMd5:p.fileMd5,
            fileSize:p.fileSize, cdnBaseUrls:p.cdnBaseUrls
          }))
        };
      }
      fs.writeFileSync(path.join(root,"packageconfig.json"),JSON.stringify(result,null,2),{flag:"wx"});
      console.log("Official package metadata received");
      return;
    } catch(e) {
      last=e;
      console.error(host + ": " + e.message);
    }
  }
  console.error("No successful official response:",last?.message);
  process.exitCode=1;
})();
