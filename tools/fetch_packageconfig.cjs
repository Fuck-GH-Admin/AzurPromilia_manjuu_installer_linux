"use strict";
const fs = require("original-fs");
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
const headers = {
  "device-id": "",
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

function get(host, endpoint, data) {
  return new Promise((resolve, reject) => {
    const timestamp = Date.now();
    const query = Object.keys(data).sort().map(k => k + "=" + data[k]).join("&");
    const sign = client.extApi3(String(timestamp) + query);
    const url = "https://" + host + endpoint + "?" + new URLSearchParams(data);
    const req = https.get(url, {
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
        if (res.statusCode !== 200) return reject(Error("HTTP " + res.statusCode + " for " + endpoint));
        try {
          const parsed = JSON.parse(raw);
          if (parsed.code !== 0) throw Error("API code " + parsed.code + " for " + endpoint);
          resolve(parsed);
        } catch (e) { reject(e); }
      });
    });
    req.setTimeout(15000, () => req.destroy(Error("request timeout for " + endpoint)));
    req.on("error", reject);
  });
}

function compactPackage(p) {
  return {
    packageType:p.packageType, downloadUrl:p.downloadUrl, fileMd5:p.fileMd5,
    fileSize:p.fileSize, cdnBaseUrls:p.cdnBaseUrls
  };
}
function compactRelease(r) {
  if (!r) return null;
  const out = {};
  for (const k of ["versionCode","versionName","gameName","gameFolderName","launchEntry","packageSize","gameExcludedFileList","domainList"]) {
    if (r[k] !== undefined) out[k] = r[k];
  }
  out.gamePackageList = (r.gamePackageList || []).map(compactPackage);
  return out;
}
function compactLaunch(data) {
  if (!data) return null;
  const out = {};
  for (const k of ["launchEntry","launchParams","gameRuntimeParams"]) {
    if (data[k] !== undefined) out[k] = data[k];
  }
  return out;
}

(async () => {
  let last;
  for (const host of ["api-grp.manjuu.com","api-grp-l2.manjuu.com"]) {
    try {
      const pkg = await get(host, "/pc-launcher/client/gameversion/packageconfig", {
        launcherId: String(config.launcherId), uid: ""
      });
      if (!pkg.data?.released?.gamePackageList?.length) throw Error("packageconfig has no released game packages");

      let gameConfig = null;
      try {
        const cfg = await get(host, "/pc-launcher/client/gameversion/config", {
          launcherId: String(config.launcherId)
        });
        gameConfig = compactRelease(cfg.data?.released || cfg.data);
      } catch (e) {
        console.error("Optional game config unavailable:", e.message);
      }

      let launchInfo = null;
      const gameVersionCode = pkg.data.released.versionCode;
      if (gameVersionCode !== undefined && gameVersionCode !== null) {
        try {
          const launch = await get(host, "/pc-launcher/client/gameversion/pre/check", {
            launcherId: String(config.launcherId), gameVersionCode: String(gameVersionCode), uid: ""
          });
          launchInfo = compactLaunch(launch.data);
        } catch (e) {
          console.error("Optional launch metadata unavailable:", e.message);
        }
      }

      const result = {
        code: 0,
        data: {
          downloadMode: pkg.data.downloadMode,
          released: compactRelease(pkg.data.released),
          preload: compactRelease(pkg.data.preload),
          gameConfig,
          launchInfo
        }
      };
      fs.writeFileSync(path.join(root,"packageconfig.json"),JSON.stringify(result,null,2),{flag:"wx"});
      console.log("Official package metadata received from", host);
      return;
    } catch(e) {
      last=e;
      console.error(host + ": " + e.message);
    }
  }
  console.error("No successful official packageconfig response:",last?.message);
  process.exitCode=1;
})();
