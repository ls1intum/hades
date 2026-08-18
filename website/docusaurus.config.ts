import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'Hades',
  tagline: 'Scalable Job Scheduler for Container Workloads',
  favicon: 'img/favicon.svg',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // Set the production url of your site here
  url: 'https://hades-scheduler.github.io',
  // Set the /<baseUrl>/ pathname under which your site is served
  baseUrl: '/hades/',
  trailingSlash: false,

  // GitHub pages deployment config.
  organizationName: 'Hades-Scheduler',
  projectName: 'hades',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'redocusaurus',
      {
        // Interactive OpenAPI reference pages generated from the committed specs.
        // The specs are synced from HadesAPI/docs and HadesLogManager/docs by
        // `make docs-site-sync`.
        specs: [
          {id: 'hades-api', spec: 'static/openapi/hades-api.json', route: '/api/hades'},
          {id: 'log-manager', spec: 'static/openapi/log-manager.json', route: '/api/log-manager'},
        ],
        theme: {primaryColor: '#FF6A3D'},
      },
    ],
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/Hades-Scheduler/hades/edit/main/website/',
        },
        blog: {
          path: 'release',
          routeBasePath: 'releases',
          blogTitle: 'Hades Releases',
          showReadingTime: true,
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'warn',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/hades-social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Hades',
      logo: {
        alt: 'Hades logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          label: 'API',
          position: 'left',
          items: [
            {label: 'HadesAPI', to: '/api/hades'},
            {label: 'Log Manager', to: '/api/log-manager'},
          ],
        },
        {to: 'releases', label: 'Releases', position: 'left'},
        {
          href: 'https://github.com/Hades-Scheduler/hades',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Introduction', to: '/docs/intro'},
            {label: 'Installation', to: '/docs/installation/docker-mode'},
            {label: 'API Reference', to: '/api/hades'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/Hades-Scheduler/hades'},
            {label: 'Releases', to: '/releases'},
            {label: 'License (MIT)', href: 'https://github.com/Hades-Scheduler/hades/blob/main/LICENSE'},
          ],
        },
      ],
      copyright: `Copyright © ${'2026'} Hades-Scheduler. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
