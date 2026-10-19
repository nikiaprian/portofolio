import fs from "fs";
import path from "path";
import { minify } from "terser";

const SOURCE_DIR = "assets/js";
const OUTPUT_DIR = "public/static/js";
const watchMode = process.argv.includes("--watch");

async function minifyFile(sourcePath) {
  try {
    const code = await fs.promises.readFile(sourcePath, "utf8");

    const result = await minify(code, {
      compress: true,
      mangle: true,
      format: {
        comments: false,
      },
    });

    if (!result.code) {
      throw new Error("Terser returned no code");
    }

    const rel = path.relative(SOURCE_DIR, sourcePath);
    const outPath = path.join(
      OUTPUT_DIR,
      rel.replace(/\.js$/i, ".min.js")
    );
    await fs.promises.mkdir(path.dirname(outPath), { recursive: true });
    await fs.promises.writeFile(outPath, result.code, "utf8");

    console.log(`✓ ${rel} -> ${path.relative(OUTPUT_DIR, outPath)}`);
  } catch (err) {
    console.error(`✗ Failed: ${sourcePath}`);
    console.error(err);
  }
}

async function cleanOutputMinFiles(dir) {
  let entries;
  try {
    entries = await fs.promises.readdir(dir, { withFileTypes: true });
  } catch (err) {
    if (err.code === "ENOENT") {
      return;
    }
    throw err;
  }

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      await cleanOutputMinFiles(fullPath);
      continue;
    }
    if (entry.isFile() && entry.name.endsWith(".min.js")) {
      await fs.promises.unlink(fullPath);
      console.log(`Deleted: ${path.relative(OUTPUT_DIR, fullPath)}`);
    }
  }
}

async function walkSource(dir) {
  const entries = await fs.promises.readdir(dir, { withFileTypes: true });

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      await walkSource(fullPath);
      continue;
    }

    if (
      entry.isFile() &&
      entry.name.endsWith(".js") &&
      !entry.name.endsWith(".min.js")
    ) {
      await minifyFile(fullPath);
    }
  }
}

async function buildAll() {
  console.log(`Minifying ${SOURCE_DIR} -> ${OUTPUT_DIR}`);
  await cleanOutputMinFiles(OUTPUT_DIR);
  await walkSource(SOURCE_DIR);
  console.log("Done.");
}

function startWatch() {
  let timer = null;

  const handleChange = (filename) => {
    if (!filename || !filename.endsWith(".js") || filename.endsWith(".min.js")) {
      return;
    }

    clearTimeout(timer);
    timer = setTimeout(async () => {
      const sourcePath = path.join(SOURCE_DIR, filename);
      try {
        await fs.promises.access(sourcePath);
        console.log(`\nChanged: ${filename}`);
        await minifyFile(sourcePath);
      } catch {
        // Ignore events for deleted or temporary files.
      }
    }, 100);
  };

  fs.watch(SOURCE_DIR, { recursive: true }, (_event, filename) => {
    handleChange(filename);
  });

  console.log(`Watching ${SOURCE_DIR} for changes...`);
}

(async () => {
  if (watchMode) {
    await buildAll();
    startWatch();
    return;
  }

  await buildAll();
})();