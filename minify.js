import fs from 'fs';
import path from 'path';
import { minify } from 'terser';

const SOURCE_DIR = 'public/static/js';

async function minifyFile(filePath) {
    try {

        const code = await fs.promises.readFile(filePath, 'utf8');

        const result = await minify(code, {
            compress: true,
            mangle: true,
            format: {
                comments: false,
            },
        });

        if (!result.code) {
            throw new Error('Terser returned no code');
        }

        const minFilePath = filePath.replace(/\.js$/i, '.min.js');
        await fs.promises.writeFile(minFilePath, result.code, 'utf8');

        console.log(`✓ ${path.basename(filePath)} -> ${path.basename(minFilePath)}`);
    } catch (err) {
        console.error(`✗ Failed: ${filePath}`);
        console.error(err);
    }
}

async function walk(dir) {
    const entries = await fs.promises.readdir(dir, { withFileTypes: true });

    // First delete existing .min.js files in this directory.
    for (const entry of entries) {
        const fullPath = path.join(dir, entry.name);

        if (entry.isFile() && entry.name.endsWith('.min.js')) {
            await fs.promises.unlink(fullPath);
            console.log(`Deleted: ${fullPath}`);
        }
    }

    // Then recurse directories and minify .js sources.
    for (const entry of entries) {
        const fullPath = path.join(dir, entry.name);

        if (entry.isDirectory()) {
            if (dir === SOURCE_DIR && entry.name === 'admin') {
                console.log(`Skipping admin folder: ${fullPath}`);
                continue;
            }

            await walk(fullPath);
            continue;
        }

        if (entry.isFile() && entry.name.endsWith('.js') && !entry.name.endsWith('.min.js')) {
            await minifyFile(fullPath);
        }
    }
}

(async () => {
    console.log(`Minifying JS files in: ${SOURCE_DIR}`);
    await walk(SOURCE_DIR);
    console.log('Done.');
})();
