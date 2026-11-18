import assert from "node:assert/strict";
import { createHash, timingSafeEqual } from "node:crypto";
import { readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import { DatabaseSync } from "node:sqlite";
import { test } from "node:test";

// Node's type stripping needs explicit extensions for the Worker's bundler imports.
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.endsWith(".ts") && /^\.\/[\w-]+$/.test(specifier)) {
      return nextResolve(specifier + ".ts", context);
    }
    return nextResolve(specifier, context);
  },
});
const { default: worker } = await import("../src/index.ts");
hooks.deregister();

const migrations = [
  "0001_init",
  "0002_self_registration",
  "0003_client_id",
  "0004_upload_kinds",
  "0005_no_device_info",
].map((name) => readFileSync(new URL(`../migrations/${name}.sql`, import.meta.url), "utf8"));
const migration0005 = migrations[4];
const timestamp = "2026-10-01T00:00:00.000Z";
const receiptId = "00000000-0000-4000-8000-000000000001";
const legacyHash = sha256("test-salt:192.0.2.1");

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

function openDatabase(t, migrationsToApply) {
  const db = new DatabaseSync(":memory:", { enableForeignKeyConstraints: true });
  t.after(() => db.close());
  for (const migration of migrationsToApply) db.exec(migration);
  return db;
}

function insertReceipt(db, id, deviceId) {
  db.prepare(`
    INSERT INTO receipts (id, device_id, source_id, sha256, zip_sha256, object_key,
      zip_bytes, lua_bytes, edition, app_version, status, created_at, client_id, kind, mode, addon)
    VALUES (?, ?, 'fixture-source', ?, ?, 'fixture-object',
      100, 200, 'retail', '0.1.13', 'queued', ?, ?, 'addon', 'delta', 'Auctionator')
  `).run(id, deviceId, sha256("fixture-lua"), sha256("fixture-zip"), timestamp, "a".repeat(32));
}

test("applies migrations through 0005 to a fresh SQLite database", (t) => {
  const db = openDatabase(t, migrations);
  assert.deepEqual(db.prepare("PRAGMA table_info(devices)").all().map((column) => column.name), [
    "id", "token_sha256", "label", "created_at", "last_seen_at", "revoked_at", "client_id",
  ]);
  assert.equal(db.prepare("SELECT name FROM sqlite_schema WHERE name = 'devices_reg_ip'").get(), undefined);
});

test("0005 normalizes legacy labels and preserves devices and receipts", (t) => {
  const db = openDatabase(t, migrations.slice(0, 4));
  const devices = [
    ["legacy-os", "companion 0.1.13 Windows", legacyHash, "companion"],
    ["legacy-version", "companion 0.1.12", legacyHash, "companion"],
    ["legacy-os-only", "companion Mac_OS-X", legacyHash, "companion"],
    ["legacy-default", "companion", legacyHash, "companion"],
    ["admin-mixed-case", "Companion QA laptop", null, "Companion QA laptop"],
    ["admin-prefix", "companion testing device", null, "companion testing device"],
    ["admin-same-label", "companion 0.1.13 Windows", null, "companion 0.1.13 Windows"],
    ["admin-custom", "Production Test Device", null, "Production Test Device"],
    ["admin-null", null, null, null],
    ["hashed-custom", "QA replacement laptop", legacyHash, "QA replacement laptop"],
    ["hashed-custom-prefix", "companion QA test laptop", legacyHash, "companion QA test laptop"],
    ["hashed-custom-punctuation", "companion QA/laptop", legacyHash, "companion QA/laptop"],
    ["hashed-custom-whitespace", "companion testing ", legacyHash, "companion testing "],
    ["hashed-similar-prefix", "companionship", legacyHash, "companionship"],
    ["hashed-null", null, legacyHash, null],
    ["empty-hash", "companion 0.1.13 Windows", "", "companion 0.1.13 Windows"],
    ["malformed-hash", "companion 0.1.13 Windows", "z".repeat(64), "companion 0.1.13 Windows"],
  ];
  const insert = db.prepare(`
    INSERT INTO devices (id, token_sha256, label, created_at, last_seen_at, revoked_at, reg_ip_hash, client_id)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `);
  const expectedLabels = new Map();
  for (const [id, label, hash, expectedLabel] of devices) {
    insert.run(id, sha256(id), label, timestamp, timestamp, timestamp, hash, "a".repeat(32));
    expectedLabels.set(id, expectedLabel);
  }
  insertReceipt(db, receiptId, "legacy-os");
  insertReceipt(db, "00000000-0000-4000-8000-000000000002", "admin-prefix");
  const devicesBefore = db.prepare("SELECT * FROM devices ORDER BY id").all();
  const receiptsBefore = db.prepare("SELECT * FROM receipts ORDER BY id").all();
  const foreignKeysBefore = db.prepare("PRAGMA foreign_key_list(receipts)").all();

  db.exec(migration0005);

  assert.equal(db.prepare("SELECT count(*) AS count FROM devices").get().count, devicesBefore.length);
  const getDevice = db.prepare("SELECT * FROM devices WHERE id = ?");
  for (const { reg_ip_hash, ...row } of devicesBefore) {
    assert.deepEqual({ ...getDevice.get(row.id) }, { ...row, label: expectedLabels.get(row.id) }, row.id);
  }
  assert.deepEqual(db.prepare("SELECT * FROM receipts ORDER BY id").all(), receiptsBefore);
  assert.deepEqual(db.prepare("PRAGMA foreign_key_list(receipts)").all(), foreignKeysBefore);
  assert.deepEqual(db.prepare("PRAGMA foreign_key_check").all(), []);
});

test("the Worker enrolls devices and authenticates an existing token before and after 0005", async (t) => {
  // Cloudflare adds this Web Crypto method for the admin bearer-token check.
  Object.defineProperty(crypto.subtle, "timingSafeEqual", { value: timingSafeEqual, configurable: true });
  t.after(() => delete crypto.subtle.timingSafeEqual);

  const db = openDatabase(t, migrations.slice(0, 4));
  const existingToken = "existing-device-token";
  db.prepare(`
    INSERT INTO devices (id, token_sha256, label, created_at, reg_ip_hash)
    VALUES ('existing-device', ?, 'companion 0.1.13 Windows', ?, ?)
  `).run(sha256(existingToken), timestamp, legacyHash);
  insertReceipt(db, receiptId, "existing-device");

  // Execute the Worker's actual SQL with a small adapter for the D1 methods these routes use.
  const limiter = { limit: async () => ({ success: true }) };
  const env = {
    ADMIN_TOKEN: "test-admin-token",
    IP_LIMITER: limiter,
    REGISTRATION_LIMITER: limiter,
    DEVICE_LIMITER: limiter,
    DB: {
      prepare(sql) {
        const statement = db.prepare(sql);
        return {
          bind(...values) {
            return {
              first: async () => statement.get(...values) ?? null,
              run: async () => ({ meta: statement.run(...values) }),
            };
          },
        };
      },
    },
  };

  async function request(path, { token, body } = {}) {
    const headers = new Headers();
    if (token) headers.set("Authorization", `Bearer ${token}`);
    if (body) headers.set("Content-Type", "application/json");
    const req = new Request(`https://worker.test${path}`, {
      headers,
      method: body ? "POST" : "GET",
      body: body ? JSON.stringify(body) : undefined,
    });
    const pending = [];
    const response = await worker.fetch(req, env, { waitUntil: (promise) => pending.push(promise) });
    await Promise.all(pending);
    return response;
  }

  const enrollments = [
    { path: "/v1/devices", body: { appVersion: "0.1.13", os: "Windows" }, label: "companion 0.1.13" },
    { path: "/v1/admin/devices", token: env.ADMIN_TOKEN, body: { label: "Companion QA laptop" }, label: "Companion QA laptop" },
  ];
  async function verifyEnrollment() {
    const deviceIds = [];
    for (const { path, token, body, label } of enrollments) {
      const response = await request(path, { token, body });
      assert.equal(response.status, 201, path);
      const { deviceId, token: deviceToken } = await response.json();
      const device = db.prepare("SELECT label, token_sha256 FROM devices WHERE id = ?").get(deviceId);
      assert.equal(device.label, label);
      assert.equal(device.token_sha256, sha256(deviceToken));
      deviceIds.push(deviceId);
    }
    const receipt = await request(`/v1/uploads/${receiptId}`, { token: existingToken });
    assert.equal(receipt.status, 200);
    assert.equal((await receipt.json()).id, receiptId);
    return deviceIds;
  }

  const preMigrationIds = await verifyEnrollment();
  for (const deviceId of preMigrationIds) {
    assert.equal(db.prepare("SELECT reg_ip_hash FROM devices WHERE id = ?").get(deviceId).reg_ip_hash, null);
  }

  db.exec(migration0005);

  await verifyEnrollment();
  assert.deepEqual(
    preMigrationIds.map((deviceId) => db.prepare("SELECT label FROM devices WHERE id = ?").get(deviceId).label),
    enrollments.map(({ label }) => label),
  );
});
