import type {
  ChatGroup,
  ChatGroupApplication,
  ChatGroupMember,
  ChatMessage,
  Conversation
} from "@/types/api";
import { getSession } from "@/utils/session";

type IMBlock = {
  userID: string;
  nickname?: string;
  faceURL?: string;
  createTime?: number;
};

type CacheNamespace =
  | "conversation"
  | "message"
  | "message_deleted"
  | "group"
  | "member"
  | "application"
  | "block";

type FallbackState = {
  version: 1;
  conversations: Conversation[];
  messages: Record<string, ChatMessage[]>;
  deletedMessageIDs: Record<string, string[]>;
  groups: ChatGroup[];
  members: Record<string, ChatGroupMember[]>;
  applications: ChatGroupApplication[];
  blocks: IMBlock[];
};

const DATABASE_NAME = "xingyu_im_v2";
const DATABASE_PATH = "_doc/xingyu_im_v2.db";
const CACHE_TABLE = "im_local_cache";
const FALLBACK_PREFIX = "im:v2:fallback:";
const MAX_FALLBACK_MESSAGES = 300;

let databaseReady: Promise<boolean> | undefined;

function ownerID() {
  return String(getSession().uid || "");
}

function validOwner(owner: string) {
  return Boolean(owner && owner !== "-9999");
}

function sqliteRuntime() {
  return (globalThis as unknown as { plus?: { sqlite?: any } }).plus?.sqlite;
}

function emptyFallback(): FallbackState {
  return {
    version: 1,
    conversations: [],
    messages: {},
    deletedMessageIDs: {},
    groups: [],
    members: {},
    applications: [],
    blocks: []
  };
}

function fallbackKey(owner: string) {
  return `${FALLBACK_PREFIX}${owner}`;
}

function readFallback(owner: string) {
  if (!validOwner(owner)) {
    return emptyFallback();
  }
  try {
    const stored = uni.getStorageSync(fallbackKey(owner)) as FallbackState | undefined;
    if (stored && stored.version === 1) {
      return stored;
    }
  } catch {
    // H5/小程序降级缓存失败时使用内存默认值。
  }
  return emptyFallback();
}

function writeFallback(owner: string, state: FallbackState) {
  if (!validOwner(owner)) {
    return;
  }
  try {
    uni.setStorageSync(fallbackKey(owner), state);
  } catch {
    // 降级缓存不能阻断 IM 主流程。
  }
}

function quote(value: unknown) {
  return `'${String(value ?? "")
    .replace(/\u0000/g, "")
    .replace(/'/g, "''")}'`;
}

function integer(value: unknown) {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) ? Math.max(0, Math.floor(parsed)) : 0;
}

function payload(value: unknown) {
  return quote(JSON.stringify(value ?? {}));
}

function openDatabase() {
  const sqlite = sqliteRuntime();
  if (!sqlite) {
    return Promise.resolve(false);
  }
  if (
    sqlite.isOpenDatabase?.({
      name: DATABASE_NAME,
      path: DATABASE_PATH
    })
  ) {
    return Promise.resolve(true);
  }
  return new Promise<boolean>((resolve) => {
    sqlite.openDatabase({
      name: DATABASE_NAME,
      path: DATABASE_PATH,
      success: () => resolve(true),
      fail: () => resolve(false)
    });
  });
}

function executeSQL(sql: string | string[]) {
  const sqlite = sqliteRuntime();
  if (!sqlite) {
    return Promise.reject(new Error("SQLite unavailable"));
  }
  return new Promise<void>((resolve, reject) => {
    sqlite.executeSql({
      name: DATABASE_NAME,
      sql,
      success: () => resolve(),
      fail: (error: unknown) => reject(error)
    });
  });
}

function selectSQL<T extends Record<string, unknown>>(sql: string) {
  const sqlite = sqliteRuntime();
  if (!sqlite) {
    return Promise.reject(new Error("SQLite unavailable"));
  }
  return new Promise<T[]>((resolve, reject) => {
    sqlite.selectSql({
      name: DATABASE_NAME,
      sql,
      success: (rows: T[]) => resolve(Array.isArray(rows) ? rows : []),
      fail: (error: unknown) => reject(error)
    });
  });
}

export function initIMDatabase() {
  if (databaseReady) {
    return databaseReady;
  }
  databaseReady = (async () => {
    if (!(await openDatabase())) {
      return false;
    }
    try {
      await executeSQL([
        `CREATE TABLE IF NOT EXISTS ${CACHE_TABLE} (
          owner_uid TEXT NOT NULL,
          namespace TEXT NOT NULL,
          parent_key TEXT NOT NULL DEFAULT '',
          item_key TEXT NOT NULL,
          sort_value INTEGER NOT NULL DEFAULT 0,
          payload TEXT NOT NULL,
          updated_at INTEGER NOT NULL DEFAULT 0,
          PRIMARY KEY (owner_uid, namespace, parent_key, item_key)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_${CACHE_TABLE}_scope
          ON ${CACHE_TABLE}(owner_uid, namespace, parent_key, sort_value DESC)`
      ]);
      return true;
    } catch {
      return false;
    }
  })();
  return databaseReady;
}

export async function closeIMDatabase() {
  const sqlite = sqliteRuntime();
  if (!sqlite) {
    databaseReady = undefined;
    return;
  }
  try {
    if (
      !sqlite.isOpenDatabase?.({
        name: DATABASE_NAME,
        path: DATABASE_PATH
      })
    ) {
      databaseReady = undefined;
      return;
    }
    await new Promise<void>((resolve) => {
      sqlite.closeDatabase({
        name: DATABASE_NAME,
        success: () => resolve(),
        fail: () => resolve()
      });
    });
  } finally {
    databaseReady = undefined;
  }
}

async function sqliteEnabled() {
  return Boolean(await initIMDatabase());
}

function upsertSQL(
  owner: string,
  namespace: CacheNamespace,
  parentKey: string,
  itemKey: string,
  sortValue: number,
  value: unknown
) {
  return `INSERT OR REPLACE INTO ${CACHE_TABLE}
    (owner_uid,namespace,parent_key,item_key,sort_value,payload,updated_at)
    VALUES(
      ${quote(owner)},${quote(namespace)},${quote(parentKey)},${quote(itemKey)},
      ${integer(sortValue)},${payload(value)},${Date.now()}
    )`;
}

async function replaceScope<T>(
  namespace: CacheNamespace,
  parentKey: string,
  items: T[],
  keyOf: (item: T) => string,
  sortOf: (item: T, index: number) => number,
  owner: string
) {
  if (!validOwner(owner) || !(await sqliteEnabled())) {
    return false;
  }
  const statements = [
    `DELETE FROM ${CACHE_TABLE}
      WHERE owner_uid=${quote(owner)}
        AND namespace=${quote(namespace)}
        AND parent_key=${quote(parentKey)}`,
    ...items
      .map((item, index) => {
        const key = keyOf(item);
        return key ? upsertSQL(owner, namespace, parentKey, key, sortOf(item, index), item) : "";
      })
      .filter(Boolean)
  ];
  try {
    await executeSQL(statements);
    return true;
  } catch {
    return false;
  }
}

async function mergeScope<T>(
  namespace: CacheNamespace,
  parentKey: string,
  items: T[],
  keyOf: (item: T) => string,
  sortOf: (item: T, index: number) => number,
  owner: string
) {
  if (!validOwner(owner) || !(await sqliteEnabled())) {
    return false;
  }
  const statements = items
    .map((item, index) => {
      const key = keyOf(item);
      return key ? upsertSQL(owner, namespace, parentKey, key, sortOf(item, index), item) : "";
    })
    .filter(Boolean);
  if (!statements.length) {
    return true;
  }
  try {
    await executeSQL(statements);
    return true;
  } catch {
    return false;
  }
}

async function readScope<T>(
  namespace: CacheNamespace,
  parentKey = "",
  options: { before?: number; limit?: number; ascending?: boolean } = {},
  owner: string = ownerID()
) {
  if (!validOwner(owner) || !(await sqliteEnabled())) {
    return undefined;
  }
  const before = integer(options.before || 0);
  const limit = Math.max(1, Math.min(1000, integer(options.limit || 500)));
  const direction = options.ascending ? "ASC" : "DESC";
  const beforeClause = before > 0 ? `AND sort_value<${before}` : "";
  try {
    const rows = await selectSQL<{ payload?: string }>(
      `SELECT payload FROM ${CACHE_TABLE}
       WHERE owner_uid=${quote(owner)}
         AND namespace=${quote(namespace)}
         AND parent_key=${quote(parentKey)}
         ${beforeClause}
       ORDER BY sort_value ${direction}, updated_at ${direction}
       LIMIT ${limit}`
    );
    return rows
      .map((row) => {
        try {
          return JSON.parse(String(row.payload || "{}")) as T;
        } catch {
          return undefined;
        }
      })
      .filter((item): item is T => Boolean(item));
  } catch {
    return undefined;
  }
}

async function deleteItem(
  namespace: CacheNamespace,
  parentKey: string,
  itemKey: string,
  owner: string
) {
  if (!validOwner(owner) || !(await sqliteEnabled())) {
    return false;
  }
  try {
    await executeSQL(
      `DELETE FROM ${CACHE_TABLE}
       WHERE owner_uid=${quote(owner)}
         AND namespace=${quote(namespace)}
         AND parent_key=${quote(parentKey)}
         AND item_key=${quote(itemKey)}`
    );
    return true;
  } catch {
    return false;
  }
}

function conversationID(item: Conversation) {
  return String(item.conversationID || item.id || "");
}

function messageID(item: ChatMessage) {
  return String(item.server_msg_id || item.client_msg_id || item.id || "");
}

function messageSort(item: ChatMessage, index: number) {
  return integer(item.sequence || item.addtime || index + 1);
}

export async function cacheIMConversations(items: Conversation[]) {
  const owner = ownerID();
  if (
    await replaceScope(
      "conversation",
      "",
      items,
      conversationID,
      (item, index) => integer(item.updated_at || item.addtime || items.length - index),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.conversations = items;
  writeFallback(owner, state);
}

export async function readCachedIMConversations() {
  const owner = ownerID();
  const cached = await readScope<Conversation>("conversation", "", { limit: 500 }, owner);
  return cached ?? readFallback(owner).conversations;
}

export async function removeCachedIMConversation(id: string) {
  const owner = ownerID();
  if (await deleteItem("conversation", "", id, owner)) {
    return;
  }
  const state = readFallback(owner);
  state.conversations = state.conversations.filter((item) => conversationID(item) !== id);
  writeFallback(owner, state);
}

export async function cacheIMMessages(conversationID: string, items: ChatMessage[]) {
  const owner = ownerID();
  if (
    await mergeScope(
      "message",
      conversationID,
      items,
      messageID,
      (item, index) => messageSort(item, index),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  const merged = new Map<string, ChatMessage>();
  (state.messages[conversationID] || []).forEach((item) => merged.set(messageID(item), item));
  items.forEach((item) => {
    const id = messageID(item);
    if (id) merged.set(id, item);
  });
  state.messages[conversationID] = [...merged.values()]
    .sort((left, right) => messageSort(left, 0) - messageSort(right, 0))
    .slice(-MAX_FALLBACK_MESSAGES);
  writeFallback(owner, state);
}

export async function readDeletedIMMessageIDs(conversationID: string) {
  const owner = ownerID();
  if (!validOwner(owner)) {
    return new Set<string>();
  }
  if (await sqliteEnabled()) {
    try {
      const rows = await selectSQL<{ item_key?: string }>(
        `SELECT item_key FROM ${CACHE_TABLE}
         WHERE owner_uid=${quote(owner)}
           AND namespace='message_deleted'
           AND parent_key=${quote(conversationID)}
         LIMIT 1000`
      );
      return new Set(rows.map((row) => String(row.item_key || "")).filter(Boolean));
    } catch {
      return new Set<string>();
    }
  }
  return new Set(readFallback(owner).deletedMessageIDs[conversationID] || []);
}

export async function readCachedIMMessages(
  conversationID: string,
  beforeSequence = 0,
  limit = 30
) {
  const owner = ownerID();
  if (!validOwner(owner)) {
    return [];
  }
  const cached = await readScope<ChatMessage>("message", conversationID, {
    before: beforeSequence,
    limit
  }, owner);
  let deleted: Set<string>;
  if (await sqliteEnabled()) {
    try {
      const rows = await selectSQL<{ item_key?: string }>(
        `SELECT item_key FROM ${CACHE_TABLE}
         WHERE owner_uid=${quote(owner)}
           AND namespace='message_deleted'
           AND parent_key=${quote(conversationID)}
         LIMIT 1000`
      );
      deleted = new Set(rows.map((row) => String(row.item_key || "")).filter(Boolean));
    } catch {
      deleted = new Set<string>();
    }
  } else {
    deleted = new Set(readFallback(owner).deletedMessageIDs[conversationID] || []);
  }
  const items =
    cached ??
    (readFallback(owner).messages[conversationID] || [])
      .filter((item) => !beforeSequence || messageSort(item, 0) < beforeSequence)
      .sort((left, right) => messageSort(right, 0) - messageSort(left, 0))
      .slice(0, limit);
  return items.filter((item) => !deleted.has(messageID(item)));
}

export async function markCachedIMMessageDeleted(conversationID: string, id: string) {
  const owner = ownerID();
  if (
    await mergeScope(
      "message_deleted",
      conversationID,
      [{ id }],
      (item) => item.id,
      () => Date.now(),
      owner
    )
  ) {
    await deleteItem("message", conversationID, id, owner);
    return;
  }
  const state = readFallback(owner);
  state.deletedMessageIDs[conversationID] = [
    ...new Set([...(state.deletedMessageIDs[conversationID] || []), id])
  ].slice(-1000);
  state.messages[conversationID] = (state.messages[conversationID] || []).filter(
    (item) => messageID(item) !== id
  );
  writeFallback(owner, state);
}

export async function removeCachedIMMessage(conversationID: string, id: string) {
  const owner = ownerID();
  if (await deleteItem("message", conversationID, id, owner)) {
    return;
  }
  const state = readFallback(owner);
  state.messages[conversationID] = (state.messages[conversationID] || []).filter(
    (item) => messageID(item) !== id
  );
  writeFallback(owner, state);
}

export async function cacheIMGroups(items: ChatGroup[]) {
  const owner = ownerID();
  if (
    await replaceScope(
      "group",
      "",
      items,
      (item) => item.groupID,
      (item, index) => integer(item.createTime || items.length - index),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.groups = items;
  writeFallback(owner, state);
}

export async function cacheIMGroup(item: ChatGroup) {
  const owner = ownerID();
  if (
    await mergeScope(
      "group",
      "",
      [item],
      (group) => group.groupID,
      (group) => integer(group.createTime || Date.now()),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.groups = state.groups.filter((group) => group.groupID !== item.groupID).concat(item);
  writeFallback(owner, state);
}

export async function readCachedIMGroups() {
  const owner = ownerID();
  const cached = await readScope<ChatGroup>("group", "", { limit: 500 }, owner);
  return cached ?? readFallback(owner).groups;
}

export async function readCachedIMGroup(groupID: string) {
  const owner = ownerID();
  const cached = await readScope<ChatGroup>("group", "", { limit: 500 }, owner);
  return (cached ?? readFallback(owner).groups).find((group) => group.groupID === groupID);
}

export async function removeCachedIMGroup(groupID: string) {
  const owner = ownerID();
  const databaseAvailable = await sqliteEnabled();
  if (databaseAvailable) {
    try {
      await executeSQL([
        `DELETE FROM ${CACHE_TABLE}
         WHERE owner_uid=${quote(owner)} AND namespace='group' AND item_key=${quote(groupID)}`,
        `DELETE FROM ${CACHE_TABLE}
         WHERE owner_uid=${quote(owner)} AND namespace='member' AND parent_key=${quote(groupID)}`
      ]);
      return;
    } catch {
      // SQLite 写入失败时继续维护当前帐号的降级缓存。
    }
  }
  const state = readFallback(owner);
  state.groups = state.groups.filter((group) => group.groupID !== groupID);
  delete state.members[groupID];
  writeFallback(owner, state);
}

export async function cacheIMGroupMembers(groupID: string, items: ChatGroupMember[]) {
  const owner = ownerID();
  if (
    await replaceScope(
      "member",
      groupID,
      items,
      (item) => item.userID,
      (item, index) => integer(item.roleLevel || 0) * 1_000_000 + items.length - index,
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.members[groupID] = items;
  writeFallback(owner, state);
}

export async function readCachedIMGroupMembers(groupID: string) {
  const owner = ownerID();
  const cached = await readScope<ChatGroupMember>("member", groupID, { limit: 1000 }, owner);
  return cached ?? readFallback(owner).members[groupID] ?? [];
}

export async function cacheIMGroupApplications(items: ChatGroupApplication[]) {
  const owner = ownerID();
  if (
    await replaceScope(
      "application",
      "",
      items,
      (item) => String(item.applicationID || `${item.groupID}:${item.userID}`),
      (item, index) => integer(item.reqTime || items.length - index),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.applications = items;
  writeFallback(owner, state);
}

export async function readCachedIMGroupApplications() {
  const owner = ownerID();
  const cached = await readScope<ChatGroupApplication>("application", "", { limit: 1000 }, owner);
  return cached ?? readFallback(owner).applications;
}

export async function cacheIMBlocks(items: IMBlock[]) {
  const owner = ownerID();
  if (
    await replaceScope(
      "block",
      "",
      items,
      (item) => item.userID,
      (item, index) => integer(item.createTime || items.length - index),
      owner
    )
  ) {
    return;
  }
  const state = readFallback(owner);
  state.blocks = items;
  writeFallback(owner, state);
}

export async function readCachedIMBlocks() {
  const owner = ownerID();
  const cached = await readScope<IMBlock>("block", "", { limit: 1000 }, owner);
  return cached ?? readFallback(owner).blocks;
}

export async function setCachedIMBlock(userID: string, blocked: boolean) {
  const owner = ownerID();
  if (blocked) {
    if (
      await mergeScope(
        "block",
        "",
        [{ userID, createTime: Date.now() }],
        (item) => item.userID,
        (item) => item.createTime,
        owner
      )
    ) {
      return;
    }
  } else if (await deleteItem("block", "", userID, owner)) {
    return;
  }
  const state = readFallback(owner);
  state.blocks = state.blocks.filter((item) => item.userID !== userID);
  if (blocked) {
    state.blocks.unshift({ userID, createTime: Date.now() });
  }
  writeFallback(owner, state);
}
