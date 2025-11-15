import type { Database } from '@tursodatabase/database';
import { getDb } from './database';
import type { Media, MediaUnit, Secret, Session, Setting, User } from '~/shared/database';
import type { RESTQuery } from '~/shared';


// Media utilities
export async function getMediaById(id: string): Promise<Media | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM media WHERE id = ?');
    const row = await stmt.get(id) as any;

    if (row) {
        // Map lowercase column names to camelCase for consistency with interface
        if (row.savetodisk !== undefined) {
            row.saveToDisk = row.savetodisk;
            delete row.savetodisk;
        }
        if (row.savedir !== undefined) {
            row.saveDir = row.savedir;
            delete row.savedir;
        }

        try {
            row.labels = JSON.parse(row.labels);
        } catch (e) {
            console.error(`Failed to parse labels for media ${id}:`, row.labels);
            row.labels = []; // Default to empty array on error
        }

        // Handle better-sqlite3's 0 to null conversion for INTEGER columns like saveToDisk
        if (row.saveToDisk === null) {
            row.saveToDisk = 0;
        } else if (row.saveToDisk !== undefined) {
            row.saveToDisk = Number(row.saveToDisk);
        }
    }
    return row as Media | undefined;
}

export async function getAllMedia(): Promise<Media[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM media ORDER BY updated_at DESC');
    const rows = await stmt.all() as any[];

    return rows.map(row => {
        try {
            row.labels = JSON.parse(row.labels);
        } catch (e) {
            console.error(`Failed to parse labels for media ${row.id}:`, row.labels);
            row.labels = []; // Default to empty array on error
        }
        return row;
    }) as Media[];
}

export async function getMediaByLabel(label: string): Promise<Media[]> {
    const db = await getDb();
    const stmt = db.prepare("SELECT * FROM media WHERE labels LIKE ?");
    const rows = await stmt.all(`%"${label}"%`) as any[]; // Search for label inside JSON array string

    return rows.map(row => {
        try {
            row.labels = JSON.parse(row.labels);
        } catch (e) {
            console.error(`Failed to parse labels for media ${row.id}:`, row.labels);
            row.labels = []; // Default to empty array on error
        }
        return row;
    }) as Media[];
}

export async function createMedia(media: Media): Promise<void> {
    const db = await getDb();
    const updatedAt = Date.now();

    const stmt = db.prepare(`
        INSERT INTO media (id, name, uri, labels, updated_at, saveToDisk, saveDir) 
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `);

    await stmt.run(
        media.id,
        media.name,
        media.uri,
        JSON.stringify(media.labels),
        updatedAt,
        media.saveToDisk === undefined ? null : media.saveToDisk,
        media.saveDir === undefined ? null : media.saveDir
    );
}

export async function updateMedia(id: string, updates: Partial<Omit<Media, 'id'>>): Promise<void> {
    const db = await getDb();

    // Build dynamic update query
    const fields = Object.keys(updates);
    if (fields.length === 0) return;

    const updatesCopy: any = { ...updates };
    if (updatesCopy.labels) {
        updatesCopy.labels = JSON.stringify(updatesCopy.labels);
    }

    const setClause = fields.map(field => `${field} = ?`).join(', ');
    const values = fields.map(field => updatesCopy[field]);

    const stmt = db.prepare(`UPDATE media SET ${setClause}, updated_at = ? WHERE id = ?`);
    await stmt.run(...values, Date.now(), id);
}

export async function deleteMedia(id: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM media WHERE id = ?');
    await stmt.run(id);
}

// Settings utilities
export async function getSetting(key: string): Promise<Setting | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM settings WHERE key = ?');
    return await stmt.get(key) as Setting | undefined;
}

export async function getAllSettings(): Promise<Setting[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM settings');
    return await stmt.all() as Setting[];
}

export async function setSetting(key: string, value: string): Promise<void> {
    const db = await getDb();

    // Try to update first, if no rows affected then insert
    const updateStmt = db.prepare('UPDATE settings SET value = ? WHERE key = ?');
    const result = await updateStmt.run(value, key);

    if (result.changes === 0) {
        // If no rows were updated, insert the new setting
        const insertStmt = db.prepare('INSERT INTO settings (key, value) VALUES (?, ?)');
        await insertStmt.run(key, value);
    }
}

export async function deleteSetting(key: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM settings WHERE key = ?');
    await stmt.run(key);
}


export async function batch_exec<T>(props: {
    db: Database;
    table: string;
    entries: T[];
    statement: string;
    // Ordered list of parameters for the prepared statement
    transform: (entry: T) => (string | number | null)[];
}) {
    console.log(`Onboarding entries into table '${props.table}'...`);
    try {
        await props.db.exec("BEGIN TRANSACTION;");

        const stmt = props.db.prepare(props.statement);
        for (const entry of props.entries) {
            const args = props.transform(entry);
            await stmt.run(...args);
        }

        await props.db.exec("COMMIT;");
        console.log(`Successfully onboarded ${props.entries.length} entries into '${props.table}'.`);

    } catch (error) {
        console.error(`Error during '${props.table}' onboarding:`, error);
        await props.db.exec("ROLLBACK;");
        throw error;
    }
}

// MediaUnit utilities
export async function createMediaUnit(mediaUnit: MediaUnit) {
    const db = await getDb();

    const stmt = db.prepare(`
        INSERT INTO media_units (id, media_id, at_time, description, embedding, path, type)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `);

    return await stmt.run(
        mediaUnit.id,
        mediaUnit.media_id,
        mediaUnit.at_time,
        mediaUnit.description || null,
        mediaUnit.embedding || null,
        mediaUnit.path,
        mediaUnit.type
    );
}

export async function getMediaUnitById(id: string): Promise<MediaUnit | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM media_units WHERE id = ?');
    return await stmt.get(id) as MediaUnit | undefined;
}

export async function getMediaUnitsByMediaId(mediaId: string): Promise<MediaUnit[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM media_units WHERE media_id = ? ORDER BY at_time ASC');
    return await stmt.all(mediaId) as MediaUnit[];
}

export async function updateMediaUnit(id: string, updates: Partial<Omit<MediaUnit, 'id'>>): Promise<void> {
    const db = await getDb();

    const fields = Object.keys(updates);
    if (fields.length === 0) return;


    const setClause = fields.map(field => `${field} = ?`).join(', ');
    const values = fields.map(field => (updates as any)[field]);

    const stmt = db.prepare(`UPDATE media_units SET ${setClause} WHERE id = ?`);
    await stmt.run(...values, id);
}

export async function deleteMediaUnit(id: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM media_units WHERE id = ?');
    await stmt.run(id);
}

// Secret utilities
export async function createSecret(key: string, value: string): Promise<string> {
    const db = await getDb();
    const stmt = db.prepare('INSERT INTO secrets (key, value) VALUES (?, ?)');
    await stmt.run(key, value);
    return key;
}

export async function getSecret(key: string): Promise<Secret | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM secrets WHERE key = ?');
    return await stmt.get(key) as Secret | undefined;
}

export async function getAllSecrets(): Promise<Secret[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM secrets');
    return await stmt.all() as Secret[];
}

export async function setSecret(key: string, value: string): Promise<void> {
    const db = await getDb();
    const updateStmt = db.prepare('UPDATE secrets SET value = ? WHERE key = ?');
    const result = await updateStmt.run(value, key);

    if (result.changes === 0) {
        const insertStmt = db.prepare('INSERT INTO secrets (key, value) VALUES (?, ?)');
        await insertStmt.run(key, value);
    }
}

export async function deleteSecret(key: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM secrets WHERE key = ?');
    await stmt.run(key);
}

// Session utilities
export async function createSession(session: Session): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare(`
        INSERT INTO sessions (session_id, user_id, created_at, expires_at)
        VALUES (?, ?, ?, ?)
    `);
    await stmt.run(session.session_id, session.user_id, session.created_at, session.expires_at);
}

export async function getSessionById(sessionId: string): Promise<Session | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM sessions WHERE session_id = ?');
    const row = await stmt.get(sessionId) as any;

    if (row) {
        // Convert timestamps to numbers if they're stored as Date objects
        if (row.created_at) row.created_at = new Date(row.created_at).getTime();
        if (row.expires_at) row.expires_at = new Date(row.expires_at).getTime();
    }

    return row as Session | undefined;
}

export async function getSessionsByUserId(userId: string): Promise<Session[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM sessions WHERE user_id = ? ORDER BY created_at DESC');
    const rows = await stmt.all(userId) as any[];

    // Convert timestamps if needed
    return rows.map(row => {
        if (row.created_at) row.created_at = new Date(row.created_at).getTime();
        if (row.expires_at) row.expires_at = new Date(row.expires_at).getTime();
        return row as Session;
    });
}

export async function updateSession(sessionId: string, updates: Partial<Omit<Session, 'session_id'>>): Promise<void> {
    const db = await getDb();
    const fields = Object.keys(updates);
    if (fields.length === 0) return;

    // Convert Date objects to timestamps if needed
    const updatesCopy: any = { ...updates };
    if (updatesCopy.created_at instanceof Date) {
        updatesCopy.created_at = updatesCopy.created_at.getTime();
    }
    if (updatesCopy.expires_at instanceof Date) {
        updatesCopy.expires_at = updatesCopy.expires_at.getTime();
    }

    const setClause = fields.map(field => `${field} = ?`).join(', ');
    const values = fields.map(field => updatesCopy[field]);

    const stmt = db.prepare(`UPDATE sessions SET ${setClause} WHERE session_id = ?`);
    await stmt.run(...values, sessionId);
}

export async function deleteSession(sessionId: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM sessions WHERE session_id = ?');
    await stmt.run(sessionId);
}

// User utilities
export async function createUser(user: User): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare(`
        INSERT INTO users (id, username, password_hash, role)
        VALUES (?, ?, ?, ?)
    `);
    await stmt.run(user.id, user.username, user.password_hash, user.role);
}

export async function getUserById(id: string): Promise<User | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM users WHERE id = ?');
    return await stmt.get(id) as User | undefined;
}

export async function getUserByUsername(username: string): Promise<User | undefined> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM users WHERE username = ?');
    return await stmt.get(username) as User | undefined;
}

export async function updateUser(id: string, updates: Partial<Omit<User, 'id'>>): Promise<void> {
    const db = await getDb();
    const fields = Object.keys(updates);
    if (fields.length === 0) return;

    const setClause = fields.map(field => `${field} = ?`).join(', ');
    const values = fields.map(field => (updates as any)[field]);

    const stmt = db.prepare(`UPDATE users SET ${setClause} WHERE id = ?`);
    await stmt.run(...values, id);
}

export async function deleteUser(id: string): Promise<void> {
    const db = await getDb();
    const stmt = db.prepare('DELETE FROM users WHERE id = ?');
    await stmt.run(id);
}

export async function getAllUsers(): Promise<User[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM users');
    return await stmt.all() as User[];
}

export async function getAllSessions(): Promise<Session[]> {
    const db = await getDb();
    const stmt = db.prepare('SELECT * FROM sessions');
    return await stmt.all() as Session[];
}

// Function to get media units by embedding (for similarity search)
export async function getMediaUnitsByEmbedding(queryEmbedding: number[]): Promise<(Omit<MediaUnit, 'embedding'> & { similarity: number })[]> {
    const db = await getDb();
    const queryEmbeddingStr = `[${queryEmbedding.join(',')}]`;
    const stmt = db.prepare(`
        SELECT 
            id, 
            media_id, 
            at_time, 
            description, 
            vector_distance_cos(embedding, vector32('${queryEmbeddingStr}')) AS similarity,
            path, 
            type
        FROM media_units 
        WHERE embedding IS NOT NULL
        ORDER BY similarity
        LIMIT 20
    `);

    // Execute the query and return results
    const rows = await stmt.all();
    return rows;
}

export async function getByQuery(query: RESTQuery): Promise<MediaUnit[]> {
    const db = await getDb();

    let sql = `SELECT * FROM ${query.table}`;
    const conditions: string[] = [];
    const values: any[] = [];

    if (query.where && query.where.length > 0) {
        for (const condition of query.where) {
            switch (condition.op) {
                case 'equals':
                    conditions.push(`${condition.field} = ?`);
                    values.push(condition.value);
                    break;
                case 'in':
                    const placeholders = (condition.value as any[]).map(() => '?').join(', ');
                    conditions.push(`${condition.field} IN (${placeholders})`);
                    values.push(...(condition.value as any[]));
                    break;
                case 'is_not':
                    conditions.push(`${condition.field} IS NOT ?`);
                    values.push(condition.value);
                    break;
                case 'like':
                    conditions.push(`${condition.field} LIKE ?`);
                    values.push(condition.value);
                    break;
                default:
                    throw new Error(`Unsupported operation: ${condition.op}`);
            }
        }
    }

    if (conditions.length > 0) {
        sql += ' WHERE ' + conditions.join(' AND ');
    }

    const stmt = db.prepare(sql);
    const rows = await stmt.all(...values) as MediaUnit[];
    return rows;
}