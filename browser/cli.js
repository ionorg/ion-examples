#!/usr/bin/env node
import _ from 'lodash'
import {buildInfo} from './lib/utils.js'
import chalk from 'chalk'
import chokidar from 'chokidar'
import CleanCSS from 'clean-css'
import connect from 'connect'
import fs from 'fs-extra'
import globby from 'globby'
import globImporter from 'node-sass-glob-importer'
import loadSettings from './lib/settings.js'
import mount from 'connect-mount'
import path from 'path'
import sass from 'node-sass'
import serveStatic from 'serve-static'
import Task from './lib/task.js'
import tinylr from 'tiny-lr'
import VuePack from '@garage11/vuepack'
import yargs from 'yargs'


const cleanCSS = new CleanCSS({level: 2, returnPromise: true, sourceMap: true})
let settings
const tasks = {}
let vuePack

// Maps tasks to entrypoints.
const entrypoint = {
    html: 'index.html',
    js: 'app.js',
    scss: 'scss/app/app.scss',
}


tasks.build = new Task('build', async function() {
    await tasks.vue.start()
    await Promise.all([
        tasks.scss.start(entrypoint.scss),
        tasks.js.start(entrypoint.js),
        tasks.html.start(entrypoint.html),
    ])
})


tasks.html = new Task('html', async function() {
    let importMap
    try {
        importMap = JSON.parse((await fs.readFile(path.join(settings.dir.build, 'lib', 'import-map.json'))))
    } catch (err) {
        importMap = {imports: []}
    }

    for (let [reference, location] of Object.entries(importMap.imports)) {
        importMap.imports[reference] = `/${path.join('lib', location)}`
    }

    const indexFile = await fs.readFile(path.join(settings.dir.app, 'index.html'))
    const compiled = _.template(indexFile)
    const html = compiled(Object.assign({settings}, {imports: importMap.imports}))

    await fs.writeFile(path.join(settings.dir.build, 'index.html'), html)
})


tasks.js = new Task('js', async function(file) {
    // Snowpack only requires a light-weight copy action to the build dir.
    let targets
    if (file) {
        await fs.copy(file, path.join(settings.dir.build, file.replace(settings.dir.base, '')))
    } else {
        targets = (await globby([
            path.join(settings.dir.app, '**', '*.js'),
            `!${path.join(settings.dir.app, 'test')}`,
        ]))

        targets.map((i) => {
            const relpath = i.replace(settings.dir.base, '')
            return fs.copy(i, path.join(settings.dir.build, relpath))
        })
        await Promise.all(targets)
    }
})


tasks.scss = new Task('scss', async function() {
    let target = {
        css: path.join(settings.dir.build, `${this.ep.filename}.css`),
        map: path.join(settings.dir.build, `${this.ep.filename}.css.map`),
    }

    return new Promise((resolve, reject) => {
        sass.render({
            file: path.join(settings.dir.app, this.ep.dirname, `${this.ep.filename}.scss`),
            importer: globImporter(),
            includePaths: [
                'node_modules',
                path.join(settings.dir.app, 'scss'),
                path.join(settings.dir.app, 'scss', 'app'),
            ],
            outFile: target.css,
            sourceMap: !settings.optimized,
            sourceMapContents: true,
            sourceMapEmbed: false,
        }, async function(err, sassObj) {
            if (err) reject(err.formatted)

            let cssRules
            const promises = []

            if (settings.optimized) {
                cssRules = (await cleanCSS.minify(sassObj.css)).styles
            } else {
                cssRules = sassObj.css
                promises.push(fs.writeFile(target.map, sassObj.map))
            }

            promises.push(fs.writeFile(target.css, cssRules))
            await Promise.all(promises)
            resolve({size: cssRules.length})
        })
    })
})


tasks.vue = new Task('vue', async function() {
    const vueFiles = await globby([path.join(settings.dir.app, 'components', '**', '*.vue')])
    if (!vuePack) {
        vuePack = new VuePack({
            basePath: settings.dir.base,
            excludeTokens: ['app', 'components'],
        })
    }

    const results = await vuePack.compile(vueFiles, this.ep ? this.ep.raw : null)

    const promises = []
    if (results.changed.components) {
        fs.writeFile(path.join(settings.dir.app, 'components.js'), results.components)
        promises.push(fs.writeFile(path.join(settings.dir.build, 'app',  'components.js'), results.components))
    }

    if (results.changed.templates) {
        // No need to wait for this write.
        fs.writeFile(path.join(settings.dir.app, 'templates.js'), results.templates)
        promises.push(fs.writeFile(path.join(settings.dir.build, 'app', 'templates.js'), results.templates))
    }
    await Promise.all(promises)
})


tasks.watch = new Task('watch', async function() {
    await tasks.build.start()
    return new Promise((resolve) => {
        var app = connect()
        console.log(settings.dir.build)
        app.use(mount('/', serveStatic(settings.dir.build)))
        app.use(async(req, res, next) => {
            if (req.url.includes('livereload.js')) {
                next()
            } else {
                const html = await fs.readFile(path.join(settings.dir.build, 'index.html'))
                res.setHeader('Content-Type', 'text/html; charset=UTF-8')
                res.end(html)
            }
        })
        app.use(tinylr.middleware({app}))
        app.listen({host: settings.dev.host, port: settings.dev.port}, () => {
            this.log(`development server listening: ${chalk.grey(`${settings.dev.host}:${settings.dev.port}`)}`)
            resolve()
        })

        chokidar.watch([
            path.join('!', settings.dir.app, 'components.js'),
            path.join('!', settings.dir.app, 'templates.js'), // Templates are handled by the Vue task
            path.join(settings.dir.app, '**', '*.js'),
        ]).on('change', async(file) => {
            await tasks.js.start(entrypoint.js, file)
            tinylr.changed('app.js')
        })

        chokidar.watch(path.join(settings.dir.app, '**', '*.vue')).on('change', async(file) => {
            await tasks.vue.start(file)
            tinylr.changed('templates.js')
        })

        chokidar.watch(path.join(settings.dir.app, '**', '*.scss')).on('change', async() => {
            await tasks.scss.start(entrypoint.scss)
            tinylr.changed('app.css')
        })

        chokidar.watch(path.join(settings.dir.app, 'index.html')).on('change', async() => {
            await tasks.html.start(entrypoint.html)
            tinylr.changed('index.html')
        })
    })
})

;(async() => {
    settings = await loadSettings()

    const cli = {
        // eslint-disable-next-line no-console
        log(...args) {console.log(...args)},
        settings,
    }

    yargs
        .usage('Usage: $0 [task]')
        .detectLocale(false)
        .option('optimized', {alias: 'o', default: false, description: 'Optimized production mode', type: 'boolean'})
        .middleware(async(argv) => {
            if (!settings.version) {
                settings.version = JSON.parse((await fs.readFile(path.join(settings.dir.base, 'package.json')))).version
            }


            // Make sure the required build directories exist.
            await fs.mkdirp(path.join(settings.dir.build, 'app'))
            settings.optimized = argv.optimized
            if (settings.optimized) {
                tasks.watch.log(`build optimization: ${chalk.green('enabled')}`)
            } else {
                tasks.watch.log(`build optimization: ${chalk.red('disabled')}`)
            }
        })

        .command('build', `build package`, () => {}, () => {tasks.build.start()})
        .command('config', 'list build config', () => {}, () => buildInfo(cli))
        .command('html', 'generate index.html', () => {}, () => {tasks.html.start(entrypoint.html)})
        .command('js', `prepare JavaScript`, () => {}, () => {tasks.js.start(entrypoint.js)})
        .command('scss', 'compile stylesheets (SCSS)', () => {}, () => {tasks.scss.start(entrypoint.scss)})
        .command('vue', 'compile Vue templates (ESM)', () => {}, () => {tasks.vue.start()})
        .command('watch', `development modus`, () => {}, () => {tasks.watch.start()})
        .demandCommand()
        .help('help')
        .showHelpOnFail(true)
        .argv


})()



