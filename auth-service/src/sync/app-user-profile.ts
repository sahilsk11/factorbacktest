import type { Pool } from "pg";

interface SyncDeps {
  pool: Pool;
  enabled: boolean;
}

interface BetterAuthUserLike {
  id: string;
  email?: string | null;
  name?: string | null;
  phoneNumber?: string | null;
}

export const buildDatabaseHooks = ({ pool, enabled }: SyncDeps) => {
  if (!enabled) {
    return undefined;
  }

  return {
    user: {
      create: {
        after: async (user: BetterAuthUserLike) => {
          try {
            await pool.query(
              `INSERT INTO public.app_user_profile (auth_user_id, email, display_name, phone_number, created_at)
               VALUES ($1, $2, $3, $4, NOW())
               ON CONFLICT (auth_user_id) DO NOTHING`,
              [user.id, user.email ?? null, user.name ?? null, user.phoneNumber ?? null],
            );
          } catch (err) {
            console.error("[app-user-sync] failed to upsert app_user_profile", err);
          }
        },
      },
      update: {
        after: async (user: BetterAuthUserLike) => {
          try {
            await pool.query(
              `UPDATE public.app_user_profile
               SET email = COALESCE($2, email),
                   display_name = COALESCE($3, display_name),
                   phone_number = COALESCE($4, phone_number),
                   updated_at = NOW()
               WHERE auth_user_id = $1`,
              [user.id, user.email ?? null, user.name ?? null, user.phoneNumber ?? null],
            );
          } catch (err) {
            console.error("[app-user-sync] failed to update app_user_profile", err);
          }
        },
      },
    },
  };
};
