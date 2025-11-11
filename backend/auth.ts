import crypto from "node:crypto";
import { table_sessions, table_users } from "./database";
import type { DbSession, DbUser } from "~/shared";

export function generateSecret(length = 64) {

    const raw = crypto.randomBytes(length);

    // base64url encode (URL-safe, no padding)
    function base64url(buffer: Buffer) {
        return buffer.toString("base64")
            .replace(/\+/g, "-")
            .replace(/\//g, "_")
            .replace(/=+$/, "");
    }

    const secret = base64url(raw);
    return secret;
}

export async function hashPassword(password: string): Promise<string> {
    return await Bun.password.hash(password, {
        algorithm: "argon2id",
        memoryCost: 4,
        timeCost: 3,
    });
}

export async function verifyPassword(password: string, hash: string): Promise<boolean> {
    return await Bun.password.verify(password, hash);
}

export async function auth_required(settings: () => Record<string, string>, req: Request): Promise<{
    data: {
        user?: DbUser,
        session?: DbSession
    },
    error?: undefined
} | {
    data?: undefined,
    error: {
        code: number,
        msg: string
    }
}> {

    if (settings().auth_enabled !== "true") {
        return {
            data: {
                // Empty
            }
        }

    }

    const cookies = req.headers.get("cookie");
    const session_id = cookies?.match(/session_id=([^;]+)/)?.[1];
    if (!session_id) return {
        error: {
            code: 401,
            msg: "No session ID"
        }
    }

    const session = await table_sessions
        .query()
        .where(`session_id = "${session_id}"`)
        .limit(1)
        .toArray()
        .then(s => s.at(0));

    if (!session || new Date(session.expires_at) < new Date())
        return {
            error: {
                code: 401,
                msg: "Invalid or expired session"
            }
        };

    const user = await table_users
        .query()
        .where(`id = "${session.user_id}"`)
        .limit(1)
        .toArray()
        .then(u => u.at(0));

    if (!user) return {
        error: {
            code: 404,
            msg: "User not found"
        }
    }

    return {
        data: {
            user,
            session
        }
    };

    // // optional: extend session on activity
    // const newExpiresAt = new Date(Date.now() + SESSION_DURATION_HOURS * 3600 * 1000);
    // await table_sessions.update(
    //     { expires_at: newExpiresAt.getTime().toString() },
    //     { where: `session_id = "${session_id}"` }
    // );
}