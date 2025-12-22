// API client for Nexus Studio

const API_BASE = '/api';

export interface TableInfo {
    name: string;
    columns?: ColumnInfo[];
}

export interface ColumnInfo {
    name: string;
    type: string;
    nullable: boolean;
    primaryKey?: boolean;
    default?: string;
}

export interface QueryResult {
    data?: Record<string, unknown>[];
    columns?: string[];
    rowsAffected?: number;
    duration?: number;
    error?: string;
}

export interface TableDataResult {
    data: Record<string, unknown>[];
    columns: string[];
    total: number;
    page: number;
    limit: number;
    pages: number;
}

export interface DbInfo {
    dialect: string;
    version: string;
}

export interface MigrationInfo {
    id: string;
    name: string;
    applied: boolean;
    appliedAt?: string;
}

// Fetch all tables
export async function getTables(): Promise<string[]> {
    const res = await fetch(`${API_BASE}/tables`);
    const data = await res.json();
    return data.tables || [];
}

// Fetch table schema
export async function getTableSchema(name: string): Promise<TableInfo> {
    const res = await fetch(`${API_BASE}/tables/${name}`);
    return res.json();
}

// Fetch table data with pagination
export async function getTableData(
    name: string,
    page = 1,
    limit = 50
): Promise<TableDataResult> {
    const res = await fetch(`${API_BASE}/tables/${name}/data?page=${page}&limit=${limit}`);
    return res.json();
}

// Fetch a single record by ID
export async function getRecordById(table: string, id: number | string): Promise<Record<string, unknown> | null> {
    const query = `SELECT * FROM ${table} WHERE id = ${id} LIMIT 1`;
    const result = await executeQuery(query);
    if (result.data && result.data.length > 0) {
        return result.data[0];
    }
    return null;
}

// Execute SQL query
export async function executeQuery(query: string): Promise<QueryResult> {
    const res = await fetch(`${API_BASE}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query })
    });
    return res.json();
}

// Insert a record
export async function insertRecord(table: string, data: Record<string, unknown>): Promise<QueryResult> {
    const columns = Object.keys(data).filter(k => data[k] !== '' && data[k] !== null);
    const values = columns.map(k => {
        const v = data[k];
        if (typeof v === 'string') return `'${v.replace(/'/g, "''")}'`;
        if (typeof v === 'boolean') return v ? '1' : '0';
        if (v === null) return 'NULL';
        return String(v);
    });

    const query = `INSERT INTO ${table} (${columns.join(', ')}) VALUES (${values.join(', ')})`;
    return executeQuery(query);
}

// Get database info
export async function getDbInfo(): Promise<DbInfo> {
    const res = await fetch(`${API_BASE}/info`);
    return res.json();
}

// Get migrations
export async function getMigrations(): Promise<MigrationInfo[]> {
    const res = await fetch(`${API_BASE}/migrations`);
    const data = await res.json();
    return data.migrations || [];
}

// Run migration action
export async function runMigration(action: 'up' | 'down'): Promise<{ message?: string; error?: string }> {
    const res = await fetch(`${API_BASE}/migrations`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action })
    });
    return res.json();
}

// Helper to detect if a column is likely a foreign key
// Returns the table name if found, null otherwise
export function isForeignKey(colName: string, availableTables: string[]): string | null {
    const match = colName.match(/^(.+)Id$/i);
    if (!match) return null;

    const baseName = match[1];
    const capitalizedBase = baseName.charAt(0).toUpperCase() + baseName.slice(1);

    // Try exact match first (postId -> Post)
    if (availableTables.includes(capitalizedBase)) {
        return capitalizedBase;
    }

    // Try common mappings (authorId -> User)
    const commonMappings: Record<string, string> = {
        'author': 'User',
        'user': 'User',
        'creator': 'User',
        'owner': 'User',
    };

    const mappedTable = commonMappings[baseName.toLowerCase()];
    if (mappedTable && availableTables.includes(mappedTable)) {
        return mappedTable;
    }

    return null;
}
