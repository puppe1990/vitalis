const { execSync } = require("node:child_process");

// #372: the kit HTML lives inside the Go module, not in web/templates, so
// Tailwind never sees its utility classes and a fresh app renders unstyled
// components. Resolve the module the same way Go does; a local replace
// (amarra-cais link) keeps working.
function shippedKitContent() {
  try {
    const out = execSync("go list -m -f {{.Dir}} github.com/puppe1990/amarra-cais", {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    });
    const dir = out.trim().replace(/\\/g, "/");
    if (!dir) {
      console.warn(
        "amarra-cais: shippedKitContent: empty module dir; kit-classes.txt still covers kit utilities. Run go mod download before npm run build (#391)."
      );
      return [];
    }
    return [dir + "/pkg/amarra/view/components/**/*.html"];
  } catch (err) {
    console.warn(
      "amarra-cais: shippedKitContent: go list -m failed; kit-classes.txt still covers kit utilities. Run go mod download before npm run build (#391).",
      err && err.message ? err.message : err
    );
    return [];
  }
}

/** @type {import("tailwindcss").Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
    "./web/static/css/kit-classes.txt",
    ...shippedKitContent(),
  ],
  safelist: [
    "cais-password-wrap",
    "cais-password-toggle",
    "cais-chat-scroll-down",
    "cais-thinking",
    "cais-thinking-dots",
    "cais-select-search",
    "cais-select-search-native",
    "cais-select-search-trigger",
    "cais-select-search-panel",
    "cais-select-search-input",
    "cais-select-search-list",
    "cais-select-search-option",
    "cais-select-search-label",
    "cais-select-search-chevron",
    "is-selected",
    "is-highlighted",
    "is-hidden",
    /* The macro bar colours are built as `vitalis-bar-{{ .Color }}` and
       `bg-{{ .Color }}`, which Tailwind's scanner cannot see, so a production
       build would drop them and render every bar as an empty track. */
    "vitalis-bar-protein",
    "vitalis-bar-carb",
    "vitalis-bar-fat",
    "bg-protein",
    "bg-carb",
    "bg-fat",
    "text-protein",
    "text-carb",
    "text-fat",
    "bg-protein-soft",
    "bg-carb-soft",
    "bg-fat-soft",
  ],
  theme: {
    extend: {
      colors: {
        /* CSS variables (input.css) so html.light can flip the palette (#258). */
        ink: "rgb(var(--amarra-ink) / <alpha-value>)",
        foam: "rgb(var(--amarra-foam) / <alpha-value>)",
        copper: "rgb(var(--amarra-copper) / <alpha-value>)",
        tide: "rgb(var(--amarra-tide) / <alpha-value>)",
        coral: "rgb(var(--amarra-coral) / <alpha-value>)",
        /* Toned semantic pair per theme — never stock red/green from Tailwind. */
        danger: "rgb(var(--amarra-danger) / <alpha-value>)",
        success: "rgb(var(--amarra-success) / <alpha-value>)",
        /* MD3 semantic roles used by the kit (<.button>, <.input>, <.flash>). */
        primary: "rgb(var(--md-primary) / <alpha-value>)",
        "on-primary": "rgb(var(--md-on-primary) / <alpha-value>)",
        "primary-fixed": "rgb(var(--md-primary-fixed) / <alpha-value>)",
        "on-primary-fixed": "rgb(var(--md-on-primary-fixed) / <alpha-value>)",
        "primary-container": "rgb(var(--md-primary-container) / <alpha-value>)",
        "on-primary-fixed-variant": "rgb(var(--md-on-primary-fixed-variant) / <alpha-value>)",
        secondary: "rgb(var(--md-secondary) / <alpha-value>)",
        "on-secondary": "rgb(var(--md-on-secondary) / <alpha-value>)",
        "secondary-container": "rgb(var(--md-secondary-container) / <alpha-value>)",
        "secondary-fixed": "rgb(var(--md-secondary-fixed) / <alpha-value>)",
        "on-secondary-fixed": "rgb(var(--md-on-secondary-fixed) / <alpha-value>)",
        tertiary: "rgb(var(--md-tertiary) / <alpha-value>)",
        "on-tertiary": "rgb(var(--md-on-tertiary) / <alpha-value>)",
        "tertiary-container": "rgb(var(--md-tertiary-container) / <alpha-value>)",
        "tertiary-fixed": "rgb(var(--md-tertiary-fixed) / <alpha-value>)",
        "on-tertiary-fixed": "rgb(var(--md-on-tertiary-fixed) / <alpha-value>)",
        error: "rgb(var(--md-error) / <alpha-value>)",
        "surface-container-lowest": "rgb(var(--md-surface-container-lowest) / <alpha-value>)",
        "surface-container-low": "rgb(var(--md-surface-container-low) / <alpha-value>)",
        "surface-container": "rgb(var(--md-surface-container) / <alpha-value>)",
        "surface-container-high": "rgb(var(--md-surface-container-high) / <alpha-value>)",
        "surface-container-highest": "rgb(var(--md-surface-container-highest) / <alpha-value>)",
        "surface-variant": "rgb(var(--md-surface-variant) / <alpha-value>)",
        "inverse-surface": "rgb(var(--md-inverse-surface) / <alpha-value>)",
        "on-surface": "rgb(var(--md-on-surface) / <alpha-value>)",
        "on-surface-variant": "rgb(var(--md-on-surface-variant) / <alpha-value>)",
        outline: "rgb(var(--md-outline) / <alpha-value>)",
        "outline-variant": "rgb(var(--md-outline-variant) / <alpha-value>)",
        /* Macronutrient roles (DESIGN.md "Macronutrient Color Roles"). */
        protein: "rgb(var(--vitalis-protein) / <alpha-value>)",
        "protein-soft": "rgb(var(--vitalis-protein-soft) / <alpha-value>)",
        carb: "rgb(var(--vitalis-carb) / <alpha-value>)",
        "carb-soft": "rgb(var(--vitalis-carb-soft) / <alpha-value>)",
        fat: "rgb(var(--vitalis-fat) / <alpha-value>)",
        "fat-soft": "rgb(var(--vitalis-fat-soft) / <alpha-value>)",
      },
      fontFamily: {
        /* Stitch "Vitalis Nutrição": one family across every level. The woff2
           files ship in web/static/fonts and are preloaded by the layout. */
        sans: ["Plus Jakarta Sans", "ui-sans-serif", "system-ui", "sans-serif"],
        serif: ["Plus Jakarta Sans", "ui-sans-serif", "system-ui", "sans-serif"],
        display: ["Plus Jakarta Sans", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
      fontSize: {
        /* Scale straight from the design doc's typography table. */
        "label-sm": ["11px", { lineHeight: "14px", letterSpacing: "0.04em", fontWeight: "700" }],
        "label-md": ["12px", { lineHeight: "16px", letterSpacing: "0.02em", fontWeight: "600" }],
        "label-lg": ["14px", { lineHeight: "18px", letterSpacing: "0.01em", fontWeight: "600" }],
        "body-sm": ["12px", { lineHeight: "18px", fontWeight: "400" }],
        "body-md": ["14px", { lineHeight: "20px", fontWeight: "400" }],
        "body-lg": ["16px", { lineHeight: "24px", fontWeight: "500" }],
        "headline-sm": ["18px", { lineHeight: "24px", fontWeight: "600" }],
        "headline-md": [
          "22px",
          { lineHeight: "28px", letterSpacing: "-0.01em", fontWeight: "700" },
        ],
        "headline-lg": [
          "28px",
          { lineHeight: "36px", letterSpacing: "-0.01em", fontWeight: "700" },
        ],
        "headline-xl": [
          "40px",
          { lineHeight: "48px", letterSpacing: "-0.02em", fontWeight: "800" },
        ],
        "headline-xl-mobile": [
          "32px",
          { lineHeight: "40px", letterSpacing: "-0.02em", fontWeight: "800" },
        ],
        "numeric-metric": [
          "36px",
          { lineHeight: "40px", letterSpacing: "-0.03em", fontWeight: "800" },
        ],
      },
      borderRadius: {
        /* Organic, hyper-rounded card architecture (DESIGN.md "Shapes"). */
        sm: "0.25rem",
        DEFAULT: "0.5rem",
        md: "0.75rem",
        lg: "1rem",
        xl: "1.5rem",
      },
      spacing: {
        "margin-mobile": "1rem",
        "space-xs": "0.25rem",
        "space-sm": "0.5rem",
        "space-md": "1rem",
        "space-lg": "1.5rem",
        "space-xl": "2rem",
      },
      boxShadow: {
        "2xs": "0 1px 2px 0 rgb(0 0 0 / 0.05)",
        xs: "0 1px 2px 0 rgb(0 0 0 / 0.05)",
      },
    },
  },
  plugins: [],
};
