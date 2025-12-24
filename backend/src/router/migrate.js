const fs = require("fs");
const path = require("path");
const os = require("os");
const { execFile } = require("child_process");
const { promisify } = require("util");

const execFileAsync = promisify(execFile);

const DEFAULT_SOURCE_DIR = process.env.SOURCE_DIR || "/vol1/1000";
const SOURCE_MAP = {
  personal: { dir: "/vol1/1000", label: "personal" },
  team: { dir: "/vol1/@team", label: "team" },
};

function normalizeBaseUrl(baseUrl) {
  return String(baseUrl || "").trim().replace(/\/+$/, "");
}

function normalizeStorage(storage) {
  return String(storage || "").trim().replace(/^\/+|\/+$/g, "");
}

async function readJson(res) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

function assertResponse(res, data, actionLabel) {
  if (!res.ok) {
    throw new Error(`${actionLabel}失败(${res.status})`);
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "success")) {
    if (data.success !== true && data.success !== 200) {
      throw new Error(data.message || data.msg || `${actionLabel}失败`);
    }
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "code")) {
    if (data.code !== 200) {
      throw new Error(data.message || data.msg || `${actionLabel}失败`);
    }
  }
}

function resolveSource(sourceType) {
  if (sourceType && SOURCE_MAP[sourceType]) {
    return SOURCE_MAP[sourceType];
  }

  if (sourceType) {
    return null;
  }

  if (DEFAULT_SOURCE_DIR === SOURCE_MAP.team.dir) {
    return SOURCE_MAP.team;
  }

  if (DEFAULT_SOURCE_DIR === SOURCE_MAP.personal.dir) {
    return SOURCE_MAP.personal;
  }

  return { dir: DEFAULT_SOURCE_DIR, label: "custom" };
}

function normalizeZipTag(tag) {
  const normalized = String(tag || "data").trim().replace(/[^a-zA-Z0-9_-]/g, "");
  return normalized || "data";
}

async function createZip(sourceDir, sourceTag) {
  if (!fs.existsSync(sourceDir)) {
    throw new Error("源目录不存在");
  }

  const stat = fs.statSync(sourceDir);
  if (!stat.isDirectory()) {
    throw new Error("源路径不是目录");
  }

  const zipName = `${normalizeZipTag(sourceTag)}-${Date.now()}.zip`;
  const zipPath = path.join(os.tmpdir(), zipName);

  if (fs.existsSync(zipPath)) {
    fs.unlinkSync(zipPath);
  }

  try {
    await execFileAsync("zip", ["-r", "-q", zipPath, "."], { cwd: sourceDir });
  } catch (error) {
    throw new Error(`压缩失败: ${error.message}`);
  }

  if (!fs.existsSync(zipPath)) {
    throw new Error("压缩文件未生成");
  }

  return { zipName, zipPath };
}

module.exports = async function migrate({ body }) {
  if (typeof fetch !== "function") {
    return { code: 500, msg: "当前环境不支持原生 fetch", data: null };
  }

  if (typeof FormData === "undefined" || typeof Blob === "undefined") {
    return { code: 500, msg: "当前环境缺少 FormData/Blob", data: null };
  }

  const baseUrl = normalizeBaseUrl(body.baseUrl);
  const username = String(body.username || "").trim();
  const password = String(body.password || "");
  const storage = normalizeStorage(body.storage);
  const sourceType = String(body.source || body.space || "").trim();

  if (!baseUrl || !username || !password) {
    return { code: 400, msg: "缺少 baseUrl/username/password", data: body };
  }

  const sourceInfo = resolveSource(sourceType);
  if (!sourceInfo) {
    return { code: 400, msg: "未知的迁移空间", data: body };
  }

  const storagePath = storage ? `/media/${storage}` : "/media";
  const dstPath = `${storagePath}`;

  let zipInfo;
  try {
    zipInfo = await createZip(sourceInfo.dir, sourceInfo.label);
  } catch (error) {
    return { code: 500, msg: error.message, data: null };
  }

  const remoteZipPath = `${storagePath}/${zipInfo.zipName}`;
  let token = "";

  try {
    const loginRes = await fetch(`${baseUrl}/v1/users/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    const loginData = await readJson(loginRes);
    assertResponse(loginRes, loginData, "登录");

    token =
      loginData?.data?.token?.access_token ||
      loginData?.data?.token ||
      loginData?.token ||
      "";

    if (!token) {
      return { code: 500, msg: "登录成功但未获取到 token", data: null };
    }

    const fileBuffer = await fs.promises.readFile(zipInfo.zipPath);
    const formData = new FormData();
    formData.append("path", storagePath);
    formData.append("rename", "");
    formData.append(
      "file",
      new Blob([fileBuffer], { type: "application/zip" }),
      zipInfo.zipName
    );

    const uploadRes = await fetch(`${baseUrl}/v2_1/files/file/uploadV2`, {
      method: "POST",
      headers: { Authorization: token },
      body: formData,
    });
    const uploadData = await readJson(uploadRes);
    assertResponse(uploadRes, uploadData, "上传");

    const decompressRes = await fetch(`${baseUrl}/v2_1/files/task/decompress`, {
      method: "POST",
      headers: {
        Authorization: token,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        src: [remoteZipPath],
        dst: dstPath,
        user_select: "overwrite",
        retain_src_file: false,
      }),
    });
    const decompressData = await readJson(decompressRes);
    assertResponse(decompressRes, decompressData, "解压");

    return {
      code: 200,
      msg: "操作成功",
      data: {
        zipPath: remoteZipPath,
        dstPath,
        sourceDir: sourceInfo.dir,
        sourceType: sourceInfo.label,
      },
    };
  } catch (error) {
    return { code: 500, msg: error.message || "迁移失败", data: null };
  } finally {
    try {
      if (zipInfo?.zipPath && fs.existsSync(zipInfo.zipPath)) {
        fs.unlinkSync(zipInfo.zipPath);
      }
    } catch {
      // ignore local cleanup errors
    }
  }
};
