import type { NodeRecord } from "@/lib/api";

export type NodeTypeMeta = {
  label: string;
  wrapClass: string;
  iconClass: string;
  searchText: string;
};

type TypeDef = {
  label: string;
  wrapClass: string;
  iconClass: string;
  aliases?: string[];
};

export type NodeTypeFilter = "all" | "folder" | "document" | "text" | "image" | "video" | "audio" | "archive" | "code" | "other";

export const nodeTypeFilterOptions: Array<{ value: NodeTypeFilter; label: string }> = [
  { value: "all", label: "All Types" },
  { value: "folder", label: "Folder" },
  { value: "document", label: "Document" },
  { value: "text", label: "Text" },
  { value: "image", label: "Image" },
  { value: "video", label: "Video" },
  { value: "audio", label: "Audio" },
  { value: "archive", label: "Archive" },
  { value: "code", label: "Code/Data" },
  { value: "other", label: "Other Files" }
];

const fallbackTypeDef: TypeDef = {
  label: "File",
  wrapClass: "bg-slate-100 dark:bg-slate-800",
  iconClass: "fa-solid fa-file-lines text-slate-500"
};

const typeByExtension: Record<string, TypeDef> = {
  pdf: { label: "PDF Document", wrapClass: "bg-red-100 dark:bg-red-950/40", iconClass: "fa-solid fa-file-pdf text-red-500" },
  fig: { label: "Figma File", wrapClass: "bg-indigo-100 dark:bg-indigo-950/40", iconClass: "fa-solid fa-compass-drafting text-indigo-500" },
  xls: { label: "Excel Workbook", wrapClass: "bg-emerald-100 dark:bg-emerald-950/40", iconClass: "fa-solid fa-file-excel text-emerald-500", aliases: ["excel"] },
  xlsx: { label: "Excel Workbook", wrapClass: "bg-emerald-100 dark:bg-emerald-950/40", iconClass: "fa-solid fa-file-excel text-emerald-500", aliases: ["excel"] },
  xlsm: { label: "Excel Macro File", wrapClass: "bg-emerald-100 dark:bg-emerald-950/40", iconClass: "fa-solid fa-file-excel text-emerald-500", aliases: ["excel"] },
  csv: { label: "CSV Spreadsheet", wrapClass: "bg-emerald-100 dark:bg-emerald-950/40", iconClass: "fa-solid fa-file-csv text-emerald-500", aliases: ["excel", "table"] },
  ods: { label: "OpenDocument Spreadsheet", wrapClass: "bg-emerald-100 dark:bg-emerald-950/40", iconClass: "fa-solid fa-file-excel text-emerald-500" },
  doc: { label: "Word Document", wrapClass: "bg-blue-100 dark:bg-blue-950/40", iconClass: "fa-solid fa-file-word text-blue-500", aliases: ["word"] },
  docx: { label: "Word Document", wrapClass: "bg-blue-100 dark:bg-blue-950/40", iconClass: "fa-solid fa-file-word text-blue-500", aliases: ["word"] },
  odt: { label: "OpenDocument Text", wrapClass: "bg-blue-100 dark:bg-blue-950/40", iconClass: "fa-solid fa-file-word text-blue-500" },
  ppt: { label: "PowerPoint Presentation", wrapClass: "bg-orange-100 dark:bg-orange-950/40", iconClass: "fa-solid fa-file-powerpoint text-orange-500", aliases: ["powerpoint"] },
  pptx: { label: "PowerPoint Presentation", wrapClass: "bg-orange-100 dark:bg-orange-950/40", iconClass: "fa-solid fa-file-powerpoint text-orange-500", aliases: ["powerpoint"] },
  odp: { label: "OpenDocument Presentation", wrapClass: "bg-orange-100 dark:bg-orange-950/40", iconClass: "fa-solid fa-file-powerpoint text-orange-500" },
  txt: { label: "Text File", wrapClass: "bg-slate-100 dark:bg-slate-800", iconClass: "fa-solid fa-file-lines text-slate-500", aliases: ["text"] },
  md: { label: "Markdown File", wrapClass: "bg-slate-100 dark:bg-slate-800", iconClass: "fa-solid fa-file-lines text-slate-500", aliases: ["markdown"] },
  log: { label: "Log File", wrapClass: "bg-slate-100 dark:bg-slate-800", iconClass: "fa-solid fa-scroll text-slate-500" },
  json: { label: "JSON Data File", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500", aliases: ["api"] },
  xml: { label: "XML Data File", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500" },
  yaml: { label: "YAML Configuration", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500" },
  yml: { label: "YAML Configuration", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500" },
  toml: { label: "TOML Configuration", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500" },
  ini: { label: "INI Configuration", wrapClass: "bg-violet-100 dark:bg-violet-950/40", iconClass: "fa-solid fa-file-code text-violet-500" },
  sql: { label: "SQL Script", wrapClass: "bg-cyan-100 dark:bg-cyan-950/40", iconClass: "fa-solid fa-database text-cyan-500" },
  zip: { label: "ZIP Archive", wrapClass: "bg-amber-100 dark:bg-amber-950/40", iconClass: "fa-solid fa-file-zipper text-amber-600", aliases: ["archive"] },
  rar: { label: "RAR Archive", wrapClass: "bg-amber-100 dark:bg-amber-950/40", iconClass: "fa-solid fa-file-zipper text-amber-600", aliases: ["archive"] },
  "7z": { label: "7Z Archive", wrapClass: "bg-amber-100 dark:bg-amber-950/40", iconClass: "fa-solid fa-file-zipper text-amber-600", aliases: ["archive"] },
  tar: { label: "TAR Archive", wrapClass: "bg-amber-100 dark:bg-amber-950/40", iconClass: "fa-solid fa-file-zipper text-amber-600", aliases: ["archive"] },
  gz: { label: "GZIP Archive", wrapClass: "bg-amber-100 dark:bg-amber-950/40", iconClass: "fa-solid fa-file-zipper text-amber-600", aliases: ["archive"] },
  png: { label: "PNG Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-file-image text-pink-500", aliases: ["image"] },
  jpg: { label: "JPEG Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-file-image text-pink-500", aliases: ["image"] },
  jpeg: { label: "JPEG Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-file-image text-pink-500", aliases: ["image"] },
  webp: { label: "WEBP Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-file-image text-pink-500", aliases: ["image"] },
  gif: { label: "GIF Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-file-image text-pink-500", aliases: ["image"] },
  svg: { label: "SVG Vector Image", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-bezier-curve text-pink-500", aliases: ["image"] },
  mp4: { label: "MP4 Video", wrapClass: "bg-fuchsia-100 dark:bg-fuchsia-950/40", iconClass: "fa-solid fa-file-video text-fuchsia-500", aliases: ["video"] },
  mov: { label: "MOV Video", wrapClass: "bg-fuchsia-100 dark:bg-fuchsia-950/40", iconClass: "fa-solid fa-file-video text-fuchsia-500", aliases: ["video"] },
  mkv: { label: "MKV Video", wrapClass: "bg-fuchsia-100 dark:bg-fuchsia-950/40", iconClass: "fa-solid fa-file-video text-fuchsia-500", aliases: ["video"] },
  avi: { label: "AVI Video", wrapClass: "bg-fuchsia-100 dark:bg-fuchsia-950/40", iconClass: "fa-solid fa-file-video text-fuchsia-500", aliases: ["video"] },
  mp3: { label: "MP3 Audio File", wrapClass: "bg-purple-100 dark:bg-purple-950/40", iconClass: "fa-solid fa-file-audio text-purple-500", aliases: ["audio"] },
  wav: { label: "WAV Audio File", wrapClass: "bg-purple-100 dark:bg-purple-950/40", iconClass: "fa-solid fa-file-audio text-purple-500", aliases: ["audio"] },
  flac: { label: "FLAC Audio File", wrapClass: "bg-purple-100 dark:bg-purple-950/40", iconClass: "fa-solid fa-file-audio text-purple-500", aliases: ["audio"] },
  m4a: { label: "M4A Audio File", wrapClass: "bg-purple-100 dark:bg-purple-950/40", iconClass: "fa-solid fa-file-audio text-purple-500", aliases: ["audio"] },
  psd: { label: "Photoshop File", wrapClass: "bg-sky-100 dark:bg-sky-950/40", iconClass: "fa-solid fa-file-image text-sky-500", aliases: ["adobe"] },
  ai: { label: "Illustrator File", wrapClass: "bg-orange-100 dark:bg-orange-950/40", iconClass: "fa-solid fa-pen-nib text-orange-500", aliases: ["adobe"] },
  xd: { label: "Adobe XD File", wrapClass: "bg-pink-100 dark:bg-pink-950/40", iconClass: "fa-solid fa-vector-square text-pink-500", aliases: ["adobe"] },
  xlf: { label: "XLIFF Translation File", wrapClass: "bg-cyan-100 dark:bg-cyan-950/40", iconClass: "fa-solid fa-language text-cyan-500", aliases: ["translation"] },
  xliff: { label: "XLIFF Translation File", wrapClass: "bg-cyan-100 dark:bg-cyan-950/40", iconClass: "fa-solid fa-language text-cyan-500", aliases: ["translation"] }
};

const editableTextExtensions = new Set([
  "txt",
  "md",
  "log",
  "csv",
  "tsv",
  "json",
  "xml",
  "yaml",
  "yml",
  "toml",
  "ini",
  "sql",
  "xlf",
  "xliff"
]);

const documentExtensions = new Set(["pdf", "doc", "docx", "ppt", "pptx", "xls", "xlsx", "xlsm", "ods", "odt", "odp"]);
const textExtensions = new Set(["txt", "md", "log", "csv", "tsv", "xlf", "xliff"]);
const codeExtensions = new Set(["json", "xml", "yaml", "yml", "toml", "ini", "sql"]);
const imageExtensions = new Set(["png", "jpg", "jpeg", "gif", "webp", "svg", "psd", "ai", "xd"]);
const videoExtensions = new Set(["mp4", "mov", "mkv", "avi"]);
const audioExtensions = new Set(["mp3", "wav", "flac", "m4a"]);
const archiveExtensions = new Set(["zip", "rar", "7z", "tar", "gz"]);

function extensionFromName(name: string): string {
  const normalized = name.trim().toLowerCase();
  const dotIndex = normalized.lastIndexOf(".");
  if (dotIndex < 0 || dotIndex === normalized.length - 1) {
    return "";
  }
  return normalized.slice(dotIndex + 1);
}

function classifyFileCategory(node: NodeRecord): Exclude<NodeTypeFilter, "all" | "folder"> {
  const extension = extensionFromName(node.name);
  const mime = (node.mime_type ?? "").toLowerCase();

  if (documentExtensions.has(extension)) {
    return "document";
  }
  if (textExtensions.has(extension)) {
    return "text";
  }
  if (codeExtensions.has(extension)) {
    return "code";
  }
  if (imageExtensions.has(extension) || mime.startsWith("image/")) {
    return "image";
  }
  if (videoExtensions.has(extension) || mime.startsWith("video/")) {
    return "video";
  }
  if (audioExtensions.has(extension) || mime.startsWith("audio/")) {
    return "audio";
  }
  if (archiveExtensions.has(extension) || mime.includes("zip") || mime.includes("compressed")) {
    return "archive";
  }
  if (mime.startsWith("text/")) {
    return "text";
  }
  return "other";
}

function inferFromMime(mime: string): TypeDef | null {
  const lower = mime.toLowerCase();
  if (lower.includes("pdf")) {
    return typeByExtension.pdf;
  }
  if (lower.includes("excel") || lower.includes("spreadsheet")) {
    return typeByExtension.xlsx;
  }
  if (lower.includes("word") || lower.includes("officedocument.wordprocessingml")) {
    return typeByExtension.docx;
  }
  if (lower.includes("powerpoint") || lower.includes("presentation")) {
    return typeByExtension.pptx;
  }
  if (lower.includes("image/")) {
    return typeByExtension.png;
  }
  if (lower.includes("video/")) {
    return typeByExtension.mp4;
  }
  if (lower.includes("audio/")) {
    return typeByExtension.mp3;
  }
  if (lower.includes("zip") || lower.includes("compressed")) {
    return typeByExtension.zip;
  }
  if (lower.includes("json")) {
    return typeByExtension.json;
  }
  if (lower.includes("xml")) {
    return typeByExtension.xml;
  }
  if (lower.includes("text/")) {
    return typeByExtension.txt;
  }
  return null;
}

function fromDefinition(definition: TypeDef): NodeTypeMeta {
  return {
    label: definition.label,
    wrapClass: definition.wrapClass,
    iconClass: definition.iconClass,
    searchText: [definition.label, ...(definition.aliases ?? [])].join(" ").toLowerCase()
  };
}

export function resolveNodeTypeMeta(node: NodeRecord): NodeTypeMeta {
  if (node.type === "folder") {
    return {
      label: "Folder",
      wrapClass: "bg-amber-100 dark:bg-amber-950/40",
      iconClass: "fa-solid fa-folder text-amber-500",
      searchText: "folder"
    };
  }

  const extension = extensionFromName(node.name);
  if (extension && typeByExtension[extension]) {
    return fromDefinition(typeByExtension[extension]);
  }

  const mime = (node.mime_type ?? "").trim();
  const inferred = mime ? inferFromMime(mime) : null;
  if (inferred) {
    return fromDefinition(inferred);
  }

  return fromDefinition(fallbackTypeDef);
}

export function resolveNodeTypeLabel(node: NodeRecord): string {
  return resolveNodeTypeMeta(node).label;
}

export function isTextEditableNode(node: NodeRecord): boolean {
  if (node.type !== "file") {
    return false;
  }
  const extension = extensionFromName(node.name);
  if (editableTextExtensions.has(extension)) {
    return true;
  }
  const mime = (node.mime_type ?? "").toLowerCase();
  if (!mime) {
    return false;
  }
  return (
    mime.startsWith("text/") ||
    mime.includes("json") ||
    mime.includes("xml") ||
    mime.includes("yaml") ||
    mime.includes("javascript") ||
    mime.includes("csv")
  );
}

export function isImagePreviewableNode(node: NodeRecord): boolean {
  if (node.type !== "file") {
    return false;
  }
  const extension = extensionFromName(node.name);
  const mime = (node.mime_type ?? "").toLowerCase();
  return imageExtensions.has(extension) || mime.startsWith("image/");
}

export function matchesNodeTypeFilter(node: NodeRecord, filter: NodeTypeFilter): boolean {
  if (filter === "all") {
    return true;
  }
  if (filter === "folder") {
    return node.type === "folder";
  }
  if (node.type !== "file") {
    return false;
  }
  return classifyFileCategory(node) === filter;
}
