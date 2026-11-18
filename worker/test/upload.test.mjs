import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import { DatabaseSync } from "node:sqlite";
import { test } from "node:test";

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

const migrations = ["0001_init", "0002_self_registration", "0003_client_id", "0004_upload_kinds", "0005_no_device_info"]
  .map((name) => readFileSync(new URL(`../migrations/${name}.sql`, import.meta.url), "utf8"));

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

function setup(t) {
  const db = new DatabaseSync(":memory:", { enableForeignKeyConstraints: true });
  t.after(() => db.close());
  for (const migration of migrations) db.exec(migration);
  const objects = new Map();
  const limiter = { limit: async () => ({ success: true }) };
  const env = {
    MAX_ZIP_BYTES: "67108864",
    MAX_LUA_BYTES: "268435456",
    IP_LIMITER: limiter,
    DEVICE_LIMITER: limiter,
    REGISTRATION_LIMITER: limiter,
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
    UPLOADS: {
      async put(key, body, options) {
        const bytes = Buffer.from(await new Response(body).arrayBuffer());
        if (options?.sha256 && sha256(bytes) !== options.sha256) throw new Error("checksum mismatch");
        objects.set(key, bytes);
        return { key, size: bytes.length };
      },
      async get(key) {
        const bytes = objects.get(key);
        return bytes ? { body: bytes, size: bytes.length } : null;
      },
      async delete(key) {
        objects.delete(key);
      },
    },
  };
  const tokens = {};
  for (const id of ["dev_victim", "dev_attacker"]) {
    tokens[id] = `${id}-token`;
    db.prepare("INSERT INTO devices (id, token_sha256, label, created_at) VALUES (?, ?, 'companion', ?)")
      .run(id, sha256(tokens[id]), "2026-11-18T00:00:00.000Z");
  }
  return { db, env, objects, tokens };
}

async function upload(env, token, zip, { clientId, sourceId, luaSha }) {
  const pending = [];
  const req = new Request("https://worker.test/v1/uploads", {
    method: "PUT",
    body: zip,
    headers: {
      authorization: `Bearer ${token}`,
      "content-type": "application/zip",
      "content-length": String(zip.length),
      "x-wowcensus-source-id": sourceId,
      "x-wowcensus-sha256": luaSha,
      "x-wowcensus-zip-sha256": sha256(zip),
      "x-wowcensus-lua-bytes": String(zip.length + 10),
      "x-wowcensus-edition": "retail",
      "x-wowcensus-app-version": "0.1.13",
      "x-wowcensus-client-id": clientId,
    },
  });
  const response = await worker.fetch(req, env, { waitUntil: (promise) => pending.push(promise) });
  await Promise.all(pending);
  return response;
}

test("another device reusing a client ID and hashes cannot replace a queued upload", async (t) => {
  const { db, env, objects, tokens } = setup(t);
  const ids = { clientId: "c".repeat(32), sourceId: sha256("source"), luaSha: sha256("lua") };
  const original = Buffer.from("victim zip bytes");
  const forged = Buffer.from("attacker zip bytes");

  const first = await upload(env, tokens.dev_victim, original, ids);
  assert.equal(first.status, 201);
  const { receiptId } = await first.json();
  const second = await upload(env, tokens.dev_attacker, forged, ids);
  assert.equal(second.status, 201);

  const victim = db.prepare("SELECT object_key, zip_sha256 FROM receipts WHERE id = ?").get(receiptId);
  assert.ok(victim.object_key.startsWith("incoming/dev_victim/"));
  assert.equal(sha256(objects.get(victim.object_key)), victim.zip_sha256);
  assert.equal(objects.size, 2);
  const keys = db.prepare("SELECT object_key FROM receipts").all().map((row) => row.object_key);
  assert.equal(new Set(keys).size, 2);
});

test("a losing attempt removes only its own object", async (t) => {
  const { db, env, objects, tokens } = setup(t);
  const ids = { clientId: "d".repeat(32), sourceId: sha256("source"), luaSha: sha256("lua") };
  const realPrepare = env.DB.prepare;
  let raced = false;
  env.DB.prepare = (sql) => {
    if (!raced && sql.startsWith("SELECT id, status FROM receipts")) {
      raced = true;
      return { bind: () => ({ first: async () => null }) };
    }
    return realPrepare(sql);
  };

  const winner = await upload(env, tokens.dev_victim, Buffer.from("winner bytes"), ids);
  assert.equal(winner.status, 201);
  raced = false;
  const loser = await upload(env, tokens.dev_victim, Buffer.from("loser bytes!"), ids);
  assert.equal(loser.status, 200);
  const body = await loser.json();
  assert.equal(body.duplicate, true);

  const row = db.prepare("SELECT object_key, zip_sha256 FROM receipts").get();
  assert.equal(db.prepare("SELECT count(*) AS n FROM receipts").get().n, 1);
  assert.deepEqual([...objects.keys()], [row.object_key]);
  assert.equal(sha256(objects.get(row.object_key)), row.zip_sha256);
});
