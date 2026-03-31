# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.5](https://github.com/rshade/finfocus/compare/v0.3.4...v0.3.5) (2026-03-31)


### Added

* **skills:** add agent skills for install, analyzer-setup, and routing ([#915](https://github.com/rshade/finfocus/issues/915)) ([7ac01d4](https://github.com/rshade/finfocus/commit/7ac01d48a83b329f7d1e21a51581ecdc3aa5b198)), closes [#909](https://github.com/rshade/finfocus/issues/909) [#910](https://github.com/rshade/finfocus/issues/910) [#912](https://github.com/rshade/finfocus/issues/912)


### Fixed

* **deps:** update go dependencies ([#927](https://github.com/rshade/finfocus/issues/927)) ([0ffa540](https://github.com/rshade/finfocus/commit/0ffa540219fcb9c68f693d1da9194cc2cc5078f2))


### Performance

* **cli:** add --state-only flag to skip pulumi preview ([#933](https://github.com/rshade/finfocus/issues/933)) ([2c27c39](https://github.com/rshade/finfocus/commit/2c27c39db2b52d3e73d3d8e4b8f5c2ec72ba9053)), closes [#690](https://github.com/rshade/finfocus/issues/690)


### Documentation

* clarify budget status visibility in overview output modes ([#932](https://github.com/rshade/finfocus/issues/932)) ([6fc93b3](https://github.com/rshade/finfocus/commit/6fc93b3953cccaffdb1efc7bb310f0a70ab1b53b)), closes [#855](https://github.com/rshade/finfocus/issues/855)

## [0.3.4](https://github.com/rshade/finfocus/compare/v0.3.3...v0.3.4) (2026-03-26)


### Added

* **cache:** consume expires_at caching hints from plugin cost responses ([#893](https://github.com/rshade/finfocus/issues/893)) ([b3979b8](https://github.com/rshade/finfocus/commit/b3979b8edec2a289476e6c3ead475f58b0514a7b))
* **router:** add BatchCost feature to capability routing ([#859](https://github.com/rshade/finfocus/issues/859)) ([648a11c](https://github.com/rshade/finfocus/commit/648a11c5844520feb8a0e41451c1982dd6aced24)), closes [#848](https://github.com/rshade/finfocus/issues/848)


### Fixed

* **cli:** preserve file sink on --debug and support qualified stack names ([#857](https://github.com/rshade/finfocus/issues/857)) ([c664f40](https://github.com/rshade/finfocus/commit/c664f404330abeff03634d5e15980b892e5d67a5))
* **deps:** update go dependencies ([#871](https://github.com/rshade/finfocus/issues/871)) ([07dd143](https://github.com/rshade/finfocus/commit/07dd1436956e80accc47f7812d5dfd21495bd458))
* **deps:** update go dependencies ([#884](https://github.com/rshade/finfocus/issues/884)) ([ce794c4](https://github.com/rshade/finfocus/commit/ce794c4baf6dfbc73e0b588cf6d08d9fccd98c13))


### Documentation

* update README badges, SEO metadata, and version references to v0.3.3 ([#907](https://github.com/rshade/finfocus/issues/907)) ([e9bc3c0](https://github.com/rshade/finfocus/commit/e9bc3c011fe49a895bbf1fa5bccf989f6112f30e))

## [0.3.3](https://github.com/rshade/finfocus/compare/v0.3.2...v0.3.3) (2026-03-03)


### Added

* **cli:** add config routes list and config routes test commands ([#840](https://github.com/rshade/finfocus/issues/840)) ([fa07e73](https://github.com/rshade/finfocus/commit/fa07e732cdefb27179b213b10a4f518d64f35198))
* **deps:** upgrade charmbracelet dependencies to v2 ([#843](https://github.com/rshade/finfocus/issues/843)) ([a1b8a97](https://github.com/rshade/finfocus/commit/a1b8a9758653dcd7dbab6dd1f56dc04d69e893af)), closes [#827](https://github.com/rshade/finfocus/issues/827)
* **tui:** show property changes in overview detail view ([#852](https://github.com/rshade/finfocus/issues/852)) ([ce24612](https://github.com/rshade/finfocus/commit/ce246122f810fbc910d814c51dcbd50ea8a57b6d))


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.224.0 ([#838](https://github.com/rshade/finfocus/issues/838)) ([d60afce](https://github.com/rshade/finfocus/commit/d60afce5d45c19079a37a0788b5451ffd9a2d11b))
* **deps:** update module github.com/rshade/finfocus-spec to v0.5.7 ([#849](https://github.com/rshade/finfocus/issues/849)) ([e0713cd](https://github.com/rshade/finfocus/commit/e0713cdd00a2f1e859076e094c75a38d4b0f3407))

## [0.3.2](https://github.com/rshade/finfocus/compare/v0.3.1...v0.3.2) (2026-02-28)


### Added

* **analyzer:** add check command, setup and stack summary ([#819](https://github.com/rshade/finfocus/issues/819)) ([353d2fa](https://github.com/rshade/finfocus/commit/353d2fa40ea0e75704dacfa5ab988dd50bc5fa86))
* **cli:** add short flags (-s, -f, -a) to overview command ([#836](https://github.com/rshade/finfocus/issues/836)) ([486125b](https://github.com/rshade/finfocus/commit/486125b127aea628c24dc9628ac937a1abb45a96)), closes [#644](https://github.com/rshade/finfocus/issues/644)
* **cli:** wire BoltDB cache into overview command ([#798](https://github.com/rshade/finfocus/issues/798)) ([4e75cd3](https://github.com/rshade/finfocus/commit/4e75cd3d961410cd74c2c0ab48f46204aea91cc5)), closes [#745](https://github.com/rshade/finfocus/issues/745)
* **overview:** display budget status and health in overview command ([#806](https://github.com/rshade/finfocus/issues/806)) ([a2565f7](https://github.com/rshade/finfocus/commit/a2565f7e82d5f5539ac5bcb6a8a6508b8f00f684))
* **overview:** state-first loading, splash screen, context-aware root ([410df5c](https://github.com/rshade/finfocus/commit/410df5ca9865234fe372b6451e839389719d923a)), closes [#728](https://github.com/rshade/finfocus/issues/728)
* **overview:** state-first loading, splash screen, context-aware root ([#730](https://github.com/rshade/finfocus/issues/730)) ([a63ef5e](https://github.com/rshade/finfocus/commit/a63ef5e6fa3a775293bde1fb3c0c76e99ebabc74))
* **pulumi:** recognize .tsx, .jsx, and go.work as Pulumi source files ([#799](https://github.com/rshade/finfocus/issues/799)) ([4917712](https://github.com/rshade/finfocus/commit/49177128bb6c8e143c56afd140c14f04980c355a)), closes [#787](https://github.com/rshade/finfocus/issues/787)
* **tui:** make table separator line extend to terminal width ([#828](https://github.com/rshade/finfocus/issues/828)) ([cd1fa7c](https://github.com/rshade/finfocus/commit/cd1fa7c88afd81a03bca4ae81a78ad47d002ede7)), closes [#718](https://github.com/rshade/finfocus/issues/718)
* **tui:** use lipgloss styles in renderInitializingView ([#729](https://github.com/rshade/finfocus/issues/729)) ([930e53f](https://github.com/rshade/finfocus/commit/930e53f8d3844e50593758b7d4691faea434f3ce)), closes [#719](https://github.com/rshade/finfocus/issues/719)


### Fixed

* **analyzer:** eliminate duplicate ResolvePolicyPackDir call in RunChecks ([#823](https://github.com/rshade/finfocus/issues/823)) ([a12cb8a](https://github.com/rshade/finfocus/commit/a12cb8accbf72ab25a3e39d6feea0c13c2d6354d)), closes [#822](https://github.com/rshade/finfocus/issues/822)
* batch bug fixes for [#723](https://github.com/rshade/finfocus/issues/723), [#747](https://github.com/rshade/finfocus/issues/747), [#748](https://github.com/rshade/finfocus/issues/748), [#749](https://github.com/rshade/finfocus/issues/749), [#750](https://github.com/rshade/finfocus/issues/750), [#751](https://github.com/rshade/finfocus/issues/751), [#752](https://github.com/rshade/finfocus/issues/752), [#753](https://github.com/rshade/finfocus/issues/753) ([#792](https://github.com/rshade/finfocus/issues/792)) ([e62b00f](https://github.com/rshade/finfocus/commit/e62b00fd4f60409df173750911f731158b48bfdb))
* **cli:** isolate CLI tests from real ~/.finfocus config and plugins ([#816](https://github.com/rshade/finfocus/issues/816)) ([d04da71](https://github.com/rshade/finfocus/commit/d04da71a56317e917c493656e82712395ebdf3c0)), closes [#809](https://github.com/rshade/finfocus/issues/809)
* **cli:** thread passphrase via subprocess env, not process-wide os.Setenv ([#770](https://github.com/rshade/finfocus/issues/770)) ([fb30a33](https://github.com/rshade/finfocus/commit/fb30a33d6d10c58c9ee3a9204d321f5e4810c5c0)), closes [#761](https://github.com/rshade/finfocus/issues/761) [#763](https://github.com/rshade/finfocus/issues/763) [#764](https://github.com/rshade/finfocus/issues/764)
* **cli:** track enriched row count incrementally for accurate audit ([#830](https://github.com/rshade/finfocus/issues/830)) ([d33018b](https://github.com/rshade/finfocus/commit/d33018bcad398696b145b028fbbbe03c2bffdea6)), closes [#720](https://github.com/rshade/finfocus/issues/720)
* **cli:** wait for enrichment goroutine before plugin cleanup in overview ([#813](https://github.com/rshade/finfocus/issues/813)) ([178d5e2](https://github.com/rshade/finfocus/commit/178d5e217fd56b06e289cb1cb96eceec82019c15)), closes [#716](https://github.com/rshade/finfocus/issues/716)
* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.223.0 ([#779](https://github.com/rshade/finfocus/issues/779)) ([316a2f9](https://github.com/rshade/finfocus/commit/316a2f9faaf9a531699a35aa45a16c25d089738d))
* disable store on compact() reopen failure to prevent panic ([#817](https://github.com/rshade/finfocus/issues/817)) ([ef86d8e](https://github.com/rshade/finfocus/commit/ef86d8e3c74e30b57796fe356da787bb1dd3c5a5))
* **engine:** handle context.Canceled and context.DeadlineExceeded in classifyError ([#826](https://github.com/rshade/finfocus/issues/826)) ([05b30de](https://github.com/rshade/finfocus/commit/05b30dee08164cb471ce93bc65f3b22b35ee33e2)), closes [#726](https://github.com/rshade/finfocus/issues/726)
* fixing review issues ([11bcf4f](https://github.com/rshade/finfocus/commit/11bcf4f4dcee0d400dbed290ce42c36b98883e71))
* **logging:** add .Ctx(ctx) to log calls in changedetect.go ([#769](https://github.com/rshade/finfocus/issues/769)) ([ed39ee8](https://github.com/rshade/finfocus/commit/ed39ee87c6d9dc337c5ce1a62412893b8ad83023)), closes [#765](https://github.com/rshade/finfocus/issues/765)
* **overview:** correct drift extrapolation for mid-month resources ([#832](https://github.com/rshade/finfocus/issues/832)) ([fcd5530](https://github.com/rshade/finfocus/commit/fcd55307ac66451dd55948b217e8b1937186e5e7)), closes [#760](https://github.com/rshade/finfocus/issues/760)
* **recorder:** implement GetPricingSpec and EstimateCost methods ([#834](https://github.com/rshade/finfocus/issues/834)) ([1d0b889](https://github.com/rshade/finfocus/commit/1d0b889ac9559cf3848b342a741d1d31e6a90cde)), closes [#734](https://github.com/rshade/finfocus/issues/734)
* respect --yes flag when change detection fails in overview TUI ([#803](https://github.com/rshade/finfocus/issues/803)) ([9cc333d](https://github.com/rshade/finfocus/commit/9cc333dbaef5a2a96cc253b7331493133c4198d2)), closes [#762](https://github.com/rshade/finfocus/issues/762)
* review issues ([#767](https://github.com/rshade/finfocus/issues/767)) ([4763af2](https://github.com/rshade/finfocus/commit/4763af2d948d6a5bb86333b4c5d819b83832ab07))
* **test:** remove duplicate nil-map test and grant Claude push access ([#796](https://github.com/rshade/finfocus/issues/796)) ([0adeb1c](https://github.com/rshade/finfocus/commit/0adeb1c345d40ebb44bd6f2711d0987798acc53e)), closes [#789](https://github.com/rshade/finfocus/issues/789)
* **tui:** add state guards for init-only messages in overview model ([#829](https://github.com/rshade/finfocus/issues/829)) ([77105ac](https://github.com/rshade/finfocus/commit/77105ac026b9cfe17dab4c9a6eeb54496ab5d6b9))
* **tui:** widen Recs column to show N(-M) dismissed format ([6262b55](https://github.com/rshade/finfocus/commit/6262b555aff81ff7f33a3725910cc6d3aa1145c5))
* **tui:** widen Recs column to show N(-M) dismissed format ([887995b](https://github.com/rshade/finfocus/commit/887995b85824401a9c448a92a650544695f4f2a9)), closes [#766](https://github.com/rshade/finfocus/issues/766)
* use presence-based check for FINFOCUS_HIDE_ALIAS_HINT ([#802](https://github.com/rshade/finfocus/issues/802)) ([2e45dd2](https://github.com/rshade/finfocus/commit/2e45dd2b6aab94df4554d8c22aeaf57382d88d65)), closes [#783](https://github.com/rshade/finfocus/issues/783)


### Performance

* **engine:** parallelize per-row enrichment sub-calls ([#727](https://github.com/rshade/finfocus/issues/727)) ([2acbefb](https://github.com/rshade/finfocus/commit/2acbefb55eebdf1a0e03f1bc440628022c6f73c1))


### Changed

* **cli:** extract progress constant and add goroutine comment in overview ([#831](https://github.com/rshade/finfocus/issues/831)) ([276b550](https://github.com/rshade/finfocus/commit/276b550e235b0bdd36765aa80dd20061e10725f0)), closes [#721](https://github.com/rshade/finfocus/issues/721)


### Documentation

* fix incorrect analyzers: in Pulumi.yaml claim in architecture doc ([#804](https://github.com/rshade/finfocus/issues/804)) ([7c75efc](https://github.com/rshade/finfocus/commit/7c75efcfd6b660cd41943e1535b3006f4ef04207)), closes [#758](https://github.com/rshade/finfocus/issues/758)
* **routing:** document routing limits in analyzer/policy-pack mode ([#771](https://github.com/rshade/finfocus/issues/771)) ([2cc58ce](https://github.com/rshade/finfocus/commit/2cc58ce59e8a3170c9fb1ad381bb2348c684fcd6)), closes [#759](https://github.com/rshade/finfocus/issues/759)
* v0.3.0 documentation audit fixes ([#711](https://github.com/rshade/finfocus/issues/711)) ([d4a71dc](https://github.com/rshade/finfocus/commit/d4a71dc5cfcd1041e7db1f9a842cb3e56f28f57d))

## [0.3.1](https://github.com/rshade/finfocus/compare/v0.3.0...v0.3.1) (2026-02-18)


### Added

* **logging:** add PhaseTimer and instrument overview command ([#712](https://github.com/rshade/finfocus/issues/712)) ([f721cb2](https://github.com/rshade/finfocus/commit/f721cb268f4488422b2fd87c84fc7f3c3014ed13))


### Fixed

* **cache:** append cache subdirectory to global fallback path ([#700](https://github.com/rshade/finfocus/issues/700)) ([6c3006f](https://github.com/rshade/finfocus/commit/6c3006fea9fdd50c4ae4db51e2b2364b7a45ae41)), closes [#680](https://github.com/rshade/finfocus/issues/680)


### Performance

* **tui:** launch TUI immediately with phase progress feedback ([#713](https://github.com/rshade/finfocus/issues/713)) ([2344de6](https://github.com/rshade/finfocus/commit/2344de6de32b6e7216e8bf918dffbed35d10a745))

## [0.3.0](https://github.com/rshade/finfocus/compare/v0.2.6...v0.3.0) (2026-02-17)


### Added

* **analyzer:** add cost threshold enforcement and structured cost ([#676](https://github.com/rshade/finfocus/issues/676)) ([474f525](https://github.com/rshade/finfocus/commit/474f525d9aa5e6dfc9bcb4578d4eac01abe5fe85))
* **cache:** replace JSON file cache with BoltDB backend ([#677](https://github.com/rshade/finfocus/issues/677)) ([618b718](https://github.com/rshade/finfocus/commit/618b7187b1de0cb5b237954b1401229f9ba479b2))
* **ci:** add benchmark PR reporting with benchstat comparison ([#675](https://github.com/rshade/finfocus/issues/675)) ([4615cba](https://github.com/rshade/finfocus/commit/4615cbaf1390e2ae889a7bb0701a785cbb8015e1))
* **cli:** add analyzer install and uninstall commands ([#633](https://github.com/rshade/finfocus/issues/633)) ([63d7e23](https://github.com/rshade/finfocus/commit/63d7e23fa332da950f98ae70b1b5e922ca156f6a))
* **cli:** add cost estimate command for what-if analysis ([#538](https://github.com/rshade/finfocus/issues/538)) ([bce24df](https://github.com/rshade/finfocus/commit/bce24df43166fd0cfb0aba671a0693db366b5d7b)), closes [#463](https://github.com/rshade/finfocus/issues/463)
* **cli:** add install script for one-command binary installation ([#668](https://github.com/rshade/finfocus/issues/668)) ([66c739a](https://github.com/rshade/finfocus/commit/66c739ac79c877eb5ef3d4c5d6ea7c9bb9fbf05e))
* **cli:** add recommendation dismissal and lifecycle management ([#557](https://github.com/rshade/finfocus/issues/557)) ([04e4f1a](https://github.com/rshade/finfocus/commit/04e4f1aa0981fcd6188e14309cd72c2a45a1d61c)), closes [#464](https://github.com/rshade/finfocus/issues/464)
* **cli:** add setup command for one-command bootstrap ([#650](https://github.com/rshade/finfocus/issues/650)) ([0b0e8e8](https://github.com/rshade/finfocus/commit/0b0e8e837727a301c949bbea2d47875717ead2a5))
* **cli:** add structured errors, semantic exit codes, and plugin lis… ([#647](https://github.com/rshade/finfocus/issues/647)) ([5c94e50](https://github.com/rshade/finfocus/commit/5c94e50492baece7aa91b04fa586c8465c391b8f))
* **cli:** add unified cost overview dashboard ([#509](https://github.com/rshade/finfocus/issues/509)) ([#584](https://github.com/rshade/finfocus/issues/584)) ([bccbc9d](https://github.com/rshade/finfocus/commit/bccbc9da8b5ecaa2c14b456e7b9c268b42386438))
* **cli:** automatic Pulumi project detection for cost commands ([#586](https://github.com/rshade/finfocus/issues/586)) ([2a6db87](https://github.com/rshade/finfocus/commit/2a6db873a1b8f58518a0b52bff12eb82030214f7))
* **cli:** wire router into commands for region-aware plugin selection ([#632](https://github.com/rshade/finfocus/issues/632)) ([e696591](https://github.com/rshade/finfocus/commit/e6965913a2605078ec68c9de6c97d8034166f5c2))
* **config:** split project-local and user-global .finfocus directories ([#651](https://github.com/rshade/finfocus/issues/651)) ([d29f7f9](https://github.com/rshade/finfocus/commit/d29f7f9002e591ec5408672262e5d4f824fa6f5a))
* **engine:** add tag-based filtering to BudgetFilterOptions ([#535](https://github.com/rshade/finfocus/issues/535)) ([085b689](https://github.com/rshade/finfocus/commit/085b689c7d95d602899f16d4cecc1211cf2a13f8)), closes [#532](https://github.com/rshade/finfocus/issues/532)
* **engine:** reliability and quality fixes  ([#661](https://github.com/rshade/finfocus/issues/661)) ([c3abedf](https://github.com/rshade/finfocus/commit/c3abedffdab676df4b5cec28a1a1d1f2c9d8b84e))
* **engine:** unified caching ([#660](https://github.com/rshade/finfocus/issues/660)) ([5ee1299](https://github.com/rshade/finfocus/commit/5ee12992de44d821339feee9625dae1990b16b6a))
* **registry:** add SHA256 checksum verification for plugin install ([#673](https://github.com/rshade/finfocus/issues/673)) ([b094a80](https://github.com/rshade/finfocus/commit/b094a802eb4da5a063f7d91ad77a5b2874078249))
* **router:** filter internal Pulumi resources from cost plugin routing ([#648](https://github.com/rshade/finfocus/issues/648)) ([879e8cb](https://github.com/rshade/finfocus/commit/879e8cbecffa93797e69d6a7584a06fde31f7805))
* **router:** support GCP zone normalization in normalizeToRegion ([#631](https://github.com/rshade/finfocus/issues/631)) ([3c5f69a](https://github.com/rshade/finfocus/commit/3c5f69a701a09428f97235623a902fb816a17433)), closes [#615](https://github.com/rshade/finfocus/issues/615)
* **tui:** display recommendations in resource detail view ([#585](https://github.com/rshade/finfocus/issues/585)) ([a57fcd9](https://github.com/rshade/finfocus/commit/a57fcd9eaa829fee2b66313c6502f90ac346ebe5))


### Fixed

* **ci:** grant write permissions to Claude workflow tokens ([#571](https://github.com/rshade/finfocus/issues/571)) ([427d1e4](https://github.com/rshade/finfocus/commit/427d1e4998cc5b3dc0856e942819f99466a4aba7))
* **deps:** update go dependencies ([#566](https://github.com/rshade/finfocus/issues/566)) ([e783168](https://github.com/rshade/finfocus/commit/e783168adad9e28da408d14fc456452d4b14835f))
* **deps:** update go dependencies ([#626](https://github.com/rshade/finfocus/issues/626)) ([500ced2](https://github.com/rshade/finfocus/commit/500ced2f34e079906f485ba8024bf01dac5c4b24))
* **deps:** update module github.com/charmbracelet/bubbles to v1 ([#627](https://github.com/rshade/finfocus/issues/627)) ([fc17976](https://github.com/rshade/finfocus/commit/fc179765791867ec997c3a3b2c76147c05a6ada2))
* **ingest:** pass cloud resource IDs and ARNs to plugins for actual cost lookup ([#574](https://github.com/rshade/finfocus/issues/574)) ([3bdc6ff](https://github.com/rshade/finfocus/commit/3bdc6ff3a24b0db0fb65142e6558c760df95230a)), closes [#380](https://github.com/rshade/finfocus/issues/380)
* **logging:** auto-create log directory before opening log file ([#618](https://github.com/rshade/finfocus/issues/618)) ([8b8717e](https://github.com/rshade/finfocus/commit/8b8717ea39968acd232fe04ecfc0fe24ece5d2ff)), closes [#591](https://github.com/rshade/finfocus/issues/591)
* **proto:** deep copy CostBreakdown to prevent source mutation ([#622](https://github.com/rshade/finfocus/issues/622)) ([ce45c21](https://github.com/rshade/finfocus/commit/ce45c2192dab54e94ca7c3ba245d702f7e6e7712)), closes [#614](https://github.com/rshade/finfocus/issues/614)
* **proto:** skip phantom $0 results from empty plugin responses ([#623](https://github.com/rshade/finfocus/issues/623)) ([862ead5](https://github.com/rshade/finfocus/commit/862ead53f40eb312319f47023a5ec87815594753)), closes [#593](https://github.com/rshade/finfocus/issues/593) [#595](https://github.com/rshade/finfocus/issues/595)
* **recorder:** remove ACTUAL_COSTS capability and add Supports() override ([#628](https://github.com/rshade/finfocus/issues/628)) ([d2a8b81](https://github.com/rshade/finfocus/commit/d2a8b818be2275bc4f4f956a0dd6ef1574733908)), closes [#594](https://github.com/rshade/finfocus/issues/594) [#596](https://github.com/rshade/finfocus/issues/596)
* **registry:** fall back to filesystem discovery for plugin removal ([#621](https://github.com/rshade/finfocus/issues/621)) ([156bbde](https://github.com/rshade/finfocus/commit/156bbdef7da1877e3b427369ff76fa3c2dd60f1b)), closes [#592](https://github.com/rshade/finfocus/issues/592)


### Changed

* **cli:** wrap bare error returns with descriptive context ([#634](https://github.com/rshade/finfocus/issues/634)) ([ec1c6a7](https://github.com/rshade/finfocus/commit/ec1c6a7e4023a7127a71be47df7de6efc15bcf32)), closes [#609](https://github.com/rshade/finfocus/issues/609)
* **core:** coderabbit follow-up cleanup from pulumi auto-detect PR ([#619](https://github.com/rshade/finfocus/issues/619)) ([ce1ec73](https://github.com/rshade/finfocus/commit/ce1ec73ddcfc4aaa2c829a2512e36b5bb176cc18))


### Chores

* release 0.3.0 ([#697](https://github.com/rshade/finfocus/issues/697)) ([360b061](https://github.com/rshade/finfocus/commit/360b06170fb44cc4383faf0888a65cef8e6ee41a))

## [0.2.6](https://github.com/rshade/finfocus/compare/v0.2.5...v0.2.6) (2026-02-02)


### Added

* **cli:** add flexible budget scoping (per-provider, per-type, per-tag) ([#509](https://github.com/rshade/finfocus/issues/509)) ([54b6680](https://github.com/rshade/finfocus/commit/54b6680506e087a3cd4809bd17be16e612ef7d94))
* **greenops:** add carbon emission equivalency calculations ([#515](https://github.com/rshade/finfocus/issues/515)) ([0b70143](https://github.com/rshade/finfocus/commit/0b70143e7e20b7f19a041bc09f671dcbc552f777))
* **router:** add intelligent multi-plugin routing for cost calculations ([#507](https://github.com/rshade/finfocus/issues/507)) ([3510f92](https://github.com/rshade/finfocus/commit/3510f92c10a5a27b6b0aa5e8ddb3b64fa587331c))


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.218.0 ([#530](https://github.com/rshade/finfocus/issues/530)) ([dd653f8](https://github.com/rshade/finfocus/commit/dd653f8d4b436ae1b5b2c41007ece13e1e557547))


### Documentation

* updating readme and relevant documentation for new functions ([#524](https://github.com/rshade/finfocus/issues/524)) ([bda0f35](https://github.com/rshade/finfocus/commit/bda0f35a5d16b762658ba2ee777d5dfc064e0aa1))

## [0.2.5](https://github.com/rshade/finfocus/compare/v0.2.4...v0.2.5) (2026-01-30)


### Added

* **cli:** add budget threshold exit codes for CI/CD integration ([#496](https://github.com/rshade/finfocus/issues/496)) ([a5883ea](https://github.com/rshade/finfocus/commit/a5883ea6bf65673606e09aa045f6f06794fefdf1)), closes [#219](https://github.com/rshade/finfocus/issues/219)
* **cli:** add pagination and NDJSON streaming for CI/CD integration ([#488](https://github.com/rshade/finfocus/issues/488)) ([7026346](https://github.com/rshade/finfocus/commit/7026346cab6db708817b1450593113c9c9ebac8c)), closes [#122](https://github.com/rshade/finfocus/issues/122)
* **engine:** add budget health suite with status tracking, forecasting, and thresholds ([#494](https://github.com/rshade/finfocus/issues/494)) ([6c09cc4](https://github.com/rshade/finfocus/commit/6c09cc44ee2bfc5bb54f80e565f2b62da689f12a)), closes [#263](https://github.com/rshade/finfocus/issues/263) [#267](https://github.com/rshade/finfocus/issues/267)


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.217.0 ([#500](https://github.com/rshade/finfocus/issues/500)) ([ee3bfca](https://github.com/rshade/finfocus/commit/ee3bfcaec88d28d9acce44a2e1c26ea9a0aab3e0))
* **deps:** update module github.com/rshade/finfocus-spec to v0.5.4 ([#477](https://github.com/rshade/finfocus/issues/477)) ([4b2424c](https://github.com/rshade/finfocus/commit/4b2424c02666c48e33105d3019fcbb115108d238))


### Changed

* add ConvertToProto and ConvertValueToString helpers for gRPC plugin communication ([#520](https://github.com/rshade/finfocus/issues/520)) ([5aaefc4](https://github.com/rshade/finfocus/commit/5aaefc42202846544b413a1fab6d62e8c16a7cd9))

## [0.2.4](https://github.com/rshade/finfocus/compare/v0.2.3...v0.2.4) (2026-01-21)


### Added

* **cli:** add budget status display with threshold alerts ([#466](https://github.com/rshade/finfocus/issues/466)) ([c7fee8b](https://github.com/rshade/finfocus/commit/c7fee8bd9951856e2d2ecd26b4d3cd1d9062a966))
* **cli:** complete plugin init with recorded fixtures ([#470](https://github.com/rshade/finfocus/issues/470)) ([dfa62fb](https://github.com/rshade/finfocus/commit/dfa62fb53acacfa15ee5c1defae076286f648a0e))


### Documentation

* **tui:** add budget, recommendations, and accessibility guides ([#472](https://github.com/rshade/finfocus/issues/472)) ([7d34d80](https://github.com/rshade/finfocus/commit/7d34d805e4d9f9c1b56c49d2313e1f823f6f3e27)), closes [#226](https://github.com/rshade/finfocus/issues/226) [#468](https://github.com/rshade/finfocus/issues/468) [#469](https://github.com/rshade/finfocus/issues/469)

## [0.2.3](https://github.com/rshade/finfocus/compare/v0.2.2...v0.2.3) (2026-01-19)


### Added

* **cli:** add version fallback for plugin install command ([#439](https://github.com/rshade/finfocus/issues/439)) ([29ae341](https://github.com/rshade/finfocus/commit/29ae341acfbe146117fa43644a403e6bd98eafaa)), closes [#430](https://github.com/rshade/finfocus/issues/430)
* **engine:** implement budget filtering and summary aggregation logic ([#446](https://github.com/rshade/finfocus/issues/446)) ([39ea80c](https://github.com/rshade/finfocus/commit/39ea80c5dee176986e97dee558c1a4e87fde9108))


### Fixed

* **registry:** make GitHub API tests platform-agnostic ([#453](https://github.com/rshade/finfocus/issues/453)) ([d8eac33](https://github.com/rshade/finfocus/commit/d8eac33ba963b002f72923dc9b31574d27eaf723)), closes [#452](https://github.com/rshade/finfocus/issues/452)


### Documentation

* **cli:** document --estimate-confidence flag for cost actual command ([a2684ae](https://github.com/rshade/finfocus/commit/a2684ae6fe931e273e9cbb8041349ef3b280bd14)), closes [#333](https://github.com/rshade/finfocus/issues/333)
* **core:** update documentation for E2E testing and plugin ecosystem ([#454](https://github.com/rshade/finfocus/issues/454)) ([ee8d893](https://github.com/rshade/finfocus/commit/ee8d89328a5c169a6305f1e7afe6eeca49ac2b13))
* **deployment:** expand deployment, security, config, troubleshooting, and support guides ([#441](https://github.com/rshade/finfocus/issues/441)) ([6edb8ef](https://github.com/rshade/finfocus/commit/6edb8efc4e6cb73dfe67ec6332231af8286ff1fe)), closes [#349](https://github.com/rshade/finfocus/issues/349) [#350](https://github.com/rshade/finfocus/issues/350) [#351](https://github.com/rshade/finfocus/issues/351) [#352](https://github.com/rshade/finfocus/issues/352) [#353](https://github.com/rshade/finfocus/issues/353)

## [0.2.2](https://github.com/rshade/finfocus/compare/v0.2.1...v0.2.2) (2026-01-18)


### Added

* **cli:** implement v0.2.1 developer experience improvements ([#426](https://github.com/rshade/finfocus/issues/426)) ([6de19ee](https://github.com/rshade/finfocus/commit/6de19ee1b938300c56eb58a5d7826ac3d970f13a)), closes [#115](https://github.com/rshade/finfocus/issues/115)


### Fixed

* **registry:** resolve Windows test failures and add plugin robustness improvements ([#436](https://github.com/rshade/finfocus/issues/436)) ([3338686](https://github.com/rshade/finfocus/commit/3338686c43ed469d273f7a1e1dc95478385b68b2))

## [0.2.1](https://github.com/rshade/finfocus/compare/v0.2.0...v0.2.1) (2026-01-17)


### Fixed

* **cli:** resolve plugin mode detection and date validation issues ([#418](https://github.com/rshade/finfocus/issues/418)) ([f3da648](https://github.com/rshade/finfocus/commit/f3da64825ae4dddc881ab2fba817f35da8716e46)), closes [#114](https://github.com/rshade/finfocus/issues/114)
* **test:** align JSON output tests with finfocus wrapper pattern ([#425](https://github.com/rshade/finfocus/issues/425)) ([9ac9dc2](https://github.com/rshade/finfocus/commit/9ac9dc2b03625e349ffd0405b93c1115530ff870)), closes [#424](https://github.com/rshade/finfocus/issues/424) [#417](https://github.com/rshade/finfocus/issues/417) [#414](https://github.com/rshade/finfocus/issues/414)

## [0.2.0](https://github.com/rshade/finfocus/compare/v0.1.4...v0.2.0) (2026-01-15)


### Added

* **plugin:** implement info and dry-run discovery ([#398](https://github.com/rshade/finfocus/issues/398)) ([a768d4a](https://github.com/rshade/finfocus/commit/a768d4aa0ac26aa4b10918aedfe2670cd29f1afc)), closes [#401](https://github.com/rshade/finfocus/issues/401)


### Chores

* release 0.2.0 ([#416](https://github.com/rshade/finfocus/issues/416)) ([d151885](https://github.com/rshade/finfocus/commit/d1518857008257c1f32af6766ba467896f1ddaa2))

## [0.1.4](https://github.com/rshade/finfocus/compare/v0.1.3...v0.1.4) (2026-01-10)


### Added

* **cli:** add cost recommendations command with action type filtering ([#375](https://github.com/rshade/finfocus/issues/375)) ([1d32dca](https://github.com/rshade/finfocus/commit/1d32dca6b19b5191a341d740093e26520f36328a)), closes [#298](https://github.com/rshade/finfocus/issues/298)
* **cli:** add Pulumi tool plugin mode support ([#379](https://github.com/rshade/finfocus/issues/379)) ([62bf5c7](https://github.com/rshade/finfocus/commit/62bf5c7b5ec02f4bbd2d0c4bbec97af56655e26e)), closes [#246](https://github.com/rshade/finfocus/issues/246)
* **cli:** add state-based actual cost estimation with confidence levels ([#382](https://github.com/rshade/finfocus/issues/382)) ([80f8c28](https://github.com/rshade/finfocus/commit/80f8c28164da9671cb62cf7b1efb6c2e96626211)), closes [#380](https://github.com/rshade/finfocus/issues/380)
* **cli:** enhance cost recommendations with TUI and summary mode ([#377](https://github.com/rshade/finfocus/issues/377)) ([4c900cb](https://github.com/rshade/finfocus/commit/4c900cb1e1835ad89bd25e34c404fd7bfbe61dc8)), closes [#216](https://github.com/rshade/finfocus/issues/216)
* **proto:** add pre-flight request validation using pluginsdk ([#372](https://github.com/rshade/finfocus/issues/372)) ([e53f2d6](https://github.com/rshade/finfocus/commit/e53f2d6a09496603ae2f5bac9d623c1537419083)), closes [#233](https://github.com/rshade/finfocus/issues/233)
* **registry:** auto-select latest plugin version ([#391](https://github.com/rshade/finfocus/issues/391)) ([48c4fa3](https://github.com/rshade/finfocus/commit/48c4fa36722eaaf16750ecc3c08c364fce199390))
* **tui:** add interactive cost display with Bubble Tea ([#345](https://github.com/rshade/finfocus/issues/345)) ([de8645c](https://github.com/rshade/finfocus/commit/de8645c543dc354a881f8df3b52a6ae14198cf33)), closes [#106](https://github.com/rshade/finfocus/issues/106)


### Fixed

* **deps:** update go dependencies ([#355](https://github.com/rshade/finfocus/issues/355)) ([f2694d8](https://github.com/rshade/finfocus/commit/f2694d8eef7d4f4bce5db0bc6360c7ae0d0739c8))
* **deps:** update go dependencies ([#388](https://github.com/rshade/finfocus/issues/388)) ([d893f98](https://github.com/rshade/finfocus/commit/d893f98075f88e918bcabb56c85fc9cfd74c513f))


### Documentation

* fixing markdownlint issues ([#381](https://github.com/rshade/finfocus/issues/381)) ([11e21bc](https://github.com/rshade/finfocus/commit/11e21bcb8de8062cd6bf1de08f178fbbe030d717))
* update roadmap and README for completed milestones ([#373](https://github.com/rshade/finfocus/issues/373)) ([2c8f16b](https://github.com/rshade/finfocus/commit/2c8f16b9ff48e81b776040966adb1087bc7592dc)), closes [#320](https://github.com/rshade/finfocus/issues/320)
* updating roadmap and fixing links ([#363](https://github.com/rshade/finfocus/issues/363)) ([98da1c2](https://github.com/rshade/finfocus/commit/98da1c2a3675e89e58ecbc6c27b5ca441288c908))
* updating roadmap and fixing links ([#363](https://github.com/rshade/finfocus/issues/363)) ([8e5395b](https://github.com/rshade/finfocus/commit/8e5395b75033a7c3518f577b995fb77fd57373e4))

## [0.1.3](https://github.com/rshade/finfocus/compare/v0.1.2...v0.1.3) (2025-12-27)


### Added

* add integration tests for --filter flag across cost commands ([#300](https://github.com/rshade/finfocus/issues/300)) ([efcebf6](https://github.com/rshade/finfocus/commit/efcebf60efb48f1f57704a24b738478fa8393518)), closes [#249](https://github.com/rshade/finfocus/issues/249)
* **analyzer:** add ResourceID passthrough for recommendation correlation ([#347](https://github.com/rshade/finfocus/issues/347)) ([680b80a](https://github.com/rshade/finfocus/commit/680b80af73acc657dac79d6bf012a7bf0b3af35b)), closes [#106](https://github.com/rshade/finfocus/issues/106)
* **analyzer:** implement Pulumi Analyzer plugin for zero-click cost estimation ([#229](https://github.com/rshade/finfocus/issues/229)) ([2070b05](https://github.com/rshade/finfocus/commit/2070b05513f6e9ae2580930c02abed8fec3fe790))
* **ci:** add automated nightly failure analysis workflow ([#297](https://github.com/rshade/finfocus/issues/297)) ([ab7c516](https://github.com/rshade/finfocus/commit/ab7c516a8b269f578ba309c68d1dd291ef2d00ef)), closes [#271](https://github.com/rshade/finfocus/issues/271)
* **conformance:** add plugin conformance testing framework ([#215](https://github.com/rshade/finfocus/issues/215)) ([c37cc22](https://github.com/rshade/finfocus/commit/c37cc2283919b4ba4ff736f15f42db7c18297fc5)), closes [#201](https://github.com/rshade/finfocus/issues/201)
* **e2e:** implement E2E testing framework with Pulumi Automation API ([#238](https://github.com/rshade/finfocus/issues/238)) ([ee23ff2](https://github.com/rshade/finfocus/commit/ee23ff2b19b348086e83969457c6927a787b96ac)), closes [#177](https://github.com/rshade/finfocus/issues/177)
* implement CLI filter flag with validation and integration tests ([#332](https://github.com/rshade/finfocus/issues/332)) ([b358566](https://github.com/rshade/finfocus/commit/b3585665e7192b74d6bebfaf3fe5be13c8e8d5e6))
* implement sustainability metrics and finalize plugin sdk mapping ([#315](https://github.com/rshade/finfocus/issues/315)) ([f207c53](https://github.com/rshade/finfocus/commit/f207c534fcdd4c64b5498a459529da6a19eec1fa))
* **plugin:** add reference recorder plugin for request capture and mock responses ([#293](https://github.com/rshade/finfocus/issues/293)) ([733c2f9](https://github.com/rshade/finfocus/commit/733c2f969952718ecde99ea9a8b5a64c74b6ac58))
* **tui:** add shared TUI package with Bubble Tea/Lip Gloss components ([#258](https://github.com/rshade/finfocus/issues/258)) ([e049460](https://github.com/rshade/finfocus/commit/e049460e4ccd5545f456ecf9d2051a6f0bac94f9))
* **tui:** add Spinner and Table components from bubbles library ([#341](https://github.com/rshade/finfocus/issues/341)) ([992db5a](https://github.com/rshade/finfocus/commit/992db5ab4ef20cdce6e1f5d6c1def7382ff03628))


### Fixed

* **deps:** update go dependencies ([#281](https://github.com/rshade/finfocus/issues/281)) ([73364d6](https://github.com/rshade/finfocus/commit/73364d66cf1d53512867cf203689998dcc9b3af6))
* **deps:** update go dependencies ([#314](https://github.com/rshade/finfocus/issues/314)) ([c09f298](https://github.com/rshade/finfocus/commit/c09f298281c8b7e18d47fe086dd6fb5d921fd571))
* **deps:** update module github.com/rshade/finfocus-spec to v0.4.3 ([#211](https://github.com/rshade/finfocus/issues/211)) ([4cb56d9](https://github.com/rshade/finfocus/commit/4cb56d928ab0b5887fd2fc56c182383d9eedfffe))
* **deps:** update module github.com/spf13/cobra to v1.10.2 ([#240](https://github.com/rshade/finfocus/issues/240)) ([ad3bfd7](https://github.com/rshade/finfocus/commit/ad3bfd7b92d189a912dbae3ae10bbda2067e6bf2))
* update Go version to 1.25.6 and improve plugin integration tests ([#244](https://github.com/rshade/finfocus/issues/244)) ([4f383df](https://github.com/rshade/finfocus/commit/4f383df0df1e1d4d3d23259adef8eb29d6ea41e9))


### Changed

* **pluginhost:** remove PORT env var, use --port flag only ([#295](https://github.com/rshade/finfocus/issues/295)) ([46bcdf2](https://github.com/rshade/finfocus/commit/46bcdf24b718e6f43f0d8f5cf3092d79ac35f8ec))
* **pluginsdk:** adopt pluginsdk environment variable constants ([#272](https://github.com/rshade/finfocus/issues/272)) ([8c6e616](https://github.com/rshade/finfocus/commit/8c6e616bcc33bcd79a599d9a31b218e4aa67c34c)), closes [#230](https://github.com/rshade/finfocus/issues/230)


### Documentation

* **all:** synchronize documentation with codebase features ([#257](https://github.com/rshade/finfocus/issues/257)) ([5881cdc](https://github.com/rshade/finfocus/commit/5881cdcbbd27705d35de3de285411ebcabe4b602)), closes [#256](https://github.com/rshade/finfocus/issues/256)

## [0.1.2](https://github.com/rshade/finfocus/compare/v0.1.1...v0.1.2) (2025-12-03)


### Added

* **logging:** integrate zerolog logging across all components ([#206](https://github.com/rshade/finfocus/issues/206)) ([c152d05](https://github.com/rshade/finfocus/commit/c152d0537c394ffd4a0f07554ec12116cb5dc4a2))


### Fixed

* comprehensive input validation and error handling improvements ([#196](https://github.com/rshade/finfocus/issues/196)) ([47b0e36](https://github.com/rshade/finfocus/commit/47b0e369db86f6268a5e9d0aba87ae5f77773379))
* **deps:** update module github.com/masterminds/semver/v3 to v3.4.0 ([#199](https://github.com/rshade/finfocus/issues/199)) ([be86a7e](https://github.com/rshade/finfocus/commit/be86a7ef047d938b4a2c87ad7fff8f727be693ee))
* **pluginhost:** prevent race condition in plugin port allocation ([#192](https://github.com/rshade/finfocus/issues/192)) ([42c4a0a](https://github.com/rshade/finfocus/commit/42c4a0a488a0aa3f528579640e49ba77c3198d71))

## [0.1.1](https://github.com/rshade/finfocus/compare/v0.1.0...v0.1.1) (2025-11-29)


### Added

* **pluginsdk:** add UnaryInterceptors support to ServeConfig ([#191](https://github.com/rshade/finfocus/issues/191)) ([e05757a](https://github.com/rshade/finfocus/commit/e05757ad914d0299387cb6a1377ad5d99c843653))


### Changed

* **core:** use pluginsdk from finfocus-spec ([#189](https://github.com/rshade/finfocus/issues/189)) ([23ae52e](https://github.com/rshade/finfocus/commit/23ae52e4669ba900f6e829d45c63dfb3000cdee7))

## [0.1.0](https://github.com/rshade/finfocus/compare/v0.0.1...v0.1.0) (2025-11-26)


### ⚠ BREAKING CHANGES

* remove encryption from config, use environment variables for secrets ([#149](https://github.com/rshade/finfocus/issues/149))

### Added

* adding in testing ([#155](https://github.com/rshade/finfocus/issues/155)) ([4680d9c](https://github.com/rshade/finfocus/commit/4680d9c9aab57cd8df749dd6f1518805533420a6))
* **cli:** implement plugin install/update/remove commands ([#171](https://github.com/rshade/finfocus/issues/171)) ([c93f761](https://github.com/rshade/finfocus/commit/c93f761e5181830f5b58a6790e7241358999b43e))
* complete actual cost pipeline with cross-provider aggregation t… ([#52](https://github.com/rshade/finfocus/issues/52)) ([c0b032f](https://github.com/rshade/finfocus/commit/c0b032f78531a267b4db155c2f38c35f46c4c3b2))
* complete CLI skeleton implementation with missing flags and tests ([#15](https://github.com/rshade/finfocus/issues/15)) ([994a859](https://github.com/rshade/finfocus/commit/994a859283c1736ee204c3cce745f421ef405927)), closes [#3](https://github.com/rshade/finfocus/issues/3)
* complete plugin development SDK and template system ([#54](https://github.com/rshade/finfocus/issues/54)) ([bee3dec](https://github.com/rshade/finfocus/commit/bee3dec866b9b7f37f686cfa2da10e2bbfa2699b))
* **engine,cli:** implement comprehensive error aggregation system ([#174](https://github.com/rshade/finfocus/issues/174)) ([cc31cb5](https://github.com/rshade/finfocus/commit/cc31cb54fd07d71d6df2117114a07bba200ab962))
* **engine:** implement projected cost pipeline with enhanced spec fa… ([#31](https://github.com/rshade/finfocus/issues/31)) ([2408b47](https://github.com/rshade/finfocus/commit/2408b472154b7b9d92ee09dcbe0fe128557da1a9))
* implement comprehensive actual cost pipeline with aggregation and filtering ([#36](https://github.com/rshade/finfocus/issues/36)) ([db18307](https://github.com/rshade/finfocus/commit/db18307c1ed992ee6a09417341b78bfd43b6e333))
* implement comprehensive CI/CD pipeline setup ([#20](https://github.com/rshade/finfocus/issues/20)) ([71d4a70](https://github.com/rshade/finfocus/commit/71d4a70a083a043529f8ee01ace28284e7a48d0b)), closes [#11](https://github.com/rshade/finfocus/issues/11)
* implement comprehensive configuration management system ([#37](https://github.com/rshade/finfocus/issues/37)) ([4a21a0c](https://github.com/rshade/finfocus/commit/4a21a0cf1a9c815768e90eebb831d61107554fa0))
* implement comprehensive configuration management system ([#38](https://github.com/rshade/finfocus/issues/38)) ([a06d03b](https://github.com/rshade/finfocus/commit/a06d03b4ad0f122a9d9e4967e9562add0a59c03f))
* implement comprehensive logging and error handling infrastructure ([#59](https://github.com/rshade/finfocus/issues/59)) ([615daaf](https://github.com/rshade/finfocus/commit/615daaf7bf3f1ec45b7b83603c2a70cc3d7f7ac1)), closes [#10](https://github.com/rshade/finfocus/issues/10)
* implement comprehensive testing framework and strategy ([#58](https://github.com/rshade/finfocus/issues/58)) ([c8451af](https://github.com/rshade/finfocus/commit/c8451af5f8a57b901aa15bf2287d8cf6e695a4f4))
* integrate real proto definitions from finfocus-spec ([247fd5b](https://github.com/rshade/finfocus/commit/247fd5b96e850669e4277519b367048dcb23d3e2))
* **logging:** implement zerolog distributed tracing with debug mode ([#184](https://github.com/rshade/finfocus/issues/184)) ([4be8b26](https://github.com/rshade/finfocus/commit/4be8b26290e2b9eb182082770f78f7db7f31adb9))
* **pluginsdk:** implement Supports() gRPC handler ([#165](https://github.com/rshade/finfocus/issues/165)) ([2034a52](https://github.com/rshade/finfocus/commit/2034a52f6cd8d160bfdfcbe0d94b4a9cca5020ba))


### Fixed

* add index.md for GitHub Pages landing page and fix workflow validation ([#96](https://github.com/rshade/finfocus/issues/96)) ([609e4e2](https://github.com/rshade/finfocus/commit/609e4e2df7c7b51639b21abd2f5f10081658773c))
* add proper CSS styling and layout improvements for GitHub Pages ([#107](https://github.com/rshade/finfocus/issues/107)) ([242b3d0](https://github.com/rshade/finfocus/commit/242b3d06d0138c86a827b2dc8a3edc687b5d72bb))
* add proper CSS styling and layout improvements for GitHub Pages ([#143](https://github.com/rshade/finfocus/issues/143)) ([de35bac](https://github.com/rshade/finfocus/commit/de35bacf1537c5029e8dfd0a18ca2fa6e79a887f))
* **deps:** update github.com/rshade/finfocus-spec digest to 1130a00 ([#39](https://github.com/rshade/finfocus/issues/39)) ([16112bc](https://github.com/rshade/finfocus/commit/16112bca7bb78716bd1ac4da9c323fabf10c9774))
* **deps:** update github.com/rshade/finfocus-spec digest to 241cb09 ([#32](https://github.com/rshade/finfocus/issues/32)) ([39a83d8](https://github.com/rshade/finfocus/commit/39a83d8b877be68e2cccacd51e7cc564a8abe69f))
* **deps:** update github.com/rshade/finfocus-spec digest to 35b5694 ([#79](https://github.com/rshade/finfocus/issues/79)) ([8d03c3e](https://github.com/rshade/finfocus/commit/8d03c3e2b4d7ffe26428ce1ee5012d3e2c508cb9))
* **deps:** update github.com/rshade/finfocus-spec digest to 5825eaa ([#60](https://github.com/rshade/finfocus/issues/60)) ([3bdc514](https://github.com/rshade/finfocus/commit/3bdc5144141bb05430979fd69614bbcde998cde4))
* **deps:** update github.com/rshade/finfocus-spec digest to 79d1a15 ([#53](https://github.com/rshade/finfocus/issues/53)) ([e9f4add](https://github.com/rshade/finfocus/commit/e9f4add667a4ef4ca26abb724fbfb5dc831530bc))
* **deps:** update github.com/rshade/finfocus-spec digest to a085bd2 ([#25](https://github.com/rshade/finfocus/issues/25)) ([bbf4974](https://github.com/rshade/finfocus/commit/bbf4974e6a18dc956c8e8b25a9ed95cc3203bea2))
* **deps:** update github.com/rshade/finfocus-spec digest to d9f31a6 ([#16](https://github.com/rshade/finfocus/issues/16)) ([644ba4e](https://github.com/rshade/finfocus/commit/644ba4ec5dec924a386a0a0e8613335860ed4e80))
* **deps:** update github.com/rshade/finfocus-spec digest to e3ffb28 ([#67](https://github.com/rshade/finfocus/issues/67)) ([0135b43](https://github.com/rshade/finfocus/commit/0135b4395c4e8fa98e2ed69d3c48ecb8080805a6))
* **deps:** update go dependencies ([#159](https://github.com/rshade/finfocus/issues/159)) ([b2ad29f](https://github.com/rshade/finfocus/commit/b2ad29fff1ef33a2428a851b02e043f235ea0dad))
* **deps:** update go dependencies ([#33](https://github.com/rshade/finfocus/issues/33)) ([e54dcb3](https://github.com/rshade/finfocus/commit/e54dcb39d08beeb16cbd484d547abd88037c7443))
* **deps:** update go dependencies ([#40](https://github.com/rshade/finfocus/issues/40)) ([e59e319](https://github.com/rshade/finfocus/commit/e59e319cb6b620daecbd786174b98c5004613dc3))
* **deps:** update go dependencies ([#49](https://github.com/rshade/finfocus/issues/49)) ([8b99267](https://github.com/rshade/finfocus/commit/8b99267eb48d6a6f0cbf79d6d84e82b34b1025ff))
* **deps:** update module github.com/rshade/finfocus-spec to v0.2.0 ([#167](https://github.com/rshade/finfocus/issues/167)) ([b6c9271](https://github.com/rshade/finfocus/commit/b6c92712fc62c90a476e937d4c1dc90882229eaf))
* **deps:** update module github.com/spf13/cobra to v1.9.1 ([#17](https://github.com/rshade/finfocus/issues/17)) ([2e0e8aa](https://github.com/rshade/finfocus/commit/2e0e8aaf7633dfb32e44ab999845bce595be7827))
* **deps:** update module google.golang.org/protobuf to v1.36.10 ([#61](https://github.com/rshade/finfocus/issues/61)) ([5dd8cae](https://github.com/rshade/finfocus/commit/5dd8cae604c72d646afe2adc61d3589b3ace763e))


### Changed

* remove encryption from config, use environment variables for secrets ([#149](https://github.com/rshade/finfocus/issues/149)) ([2e3a07b](https://github.com/rshade/finfocus/commit/2e3a07b6d122ef37e0cff9b9a3d025855b92881b)), closes [#99](https://github.com/rshade/finfocus/issues/99)


### Documentation

* complete Vantage plugin documentation ([#145](https://github.com/rshade/finfocus/issues/145)) ([06e6cd7](https://github.com/rshade/finfocus/commit/06e6cd70a9328bde6d6d736146fe16b088aa1f6d)), closes [#103](https://github.com/rshade/finfocus/issues/103)
* first pass at github pages ([#88](https://github.com/rshade/finfocus/issues/88)) ([ceee2f3](https://github.com/rshade/finfocus/commit/ceee2f3fb632f0d1c8960bb36fce1e111988efd3))
* ratify constitution v1.0.0 (establish governance principles) ([#152](https://github.com/rshade/finfocus/issues/152)) ([d40ac0f](https://github.com/rshade/finfocus/commit/d40ac0fab2707b1acf7a0e2ba0db87e424f4afbe))
* update constitution for docstrings ([#176](https://github.com/rshade/finfocus/issues/176)) ([5053db5](https://github.com/rshade/finfocus/commit/5053db5865b6ecf6e2ec430181a7c9445b47cdab))

## [Unreleased]

### BREAKING CHANGES

- **Removed encryption functionality from config package**: The built-in encryption system using PBKDF2 has been completely removed due to security concerns about weak key derivation. Users should now store sensitive values (API keys, credentials) as environment variables instead of in configuration files. This is the industry-standard approach for CLI tools and follows best practices for secret management.
  - Removed `EncryptValue()` and `DecryptValue()` methods from Config
  - Removed `--encrypt` flag from `finfocus config set` command
  - Removed `--decrypt` flag from `finfocus config get` command
  - Removed all encryption-related infrastructure (deriveKey, master key management)

  **Migration Guide**:
  - Remove any encrypted values from your `~/.finfocus/config.yaml`
  - Store sensitive values as environment variables using the pattern: `FINFOCUS_PLUGIN_<PLUGIN_NAME>_<KEY_NAME>`
  - Example: `export FINFOCUS_PLUGIN_AWS_SECRET_KEY="your-secret"`
  - Environment variables automatically override config file values

### Changed

- Updated CLI command documentation to recommend environment variables for sensitive data
- Updated README with comprehensive configuration and environment variable documentation
- Simplified config package by removing unused encryption dependencies

### Removed

- PBKDF2-based encryption key derivation (security vulnerability)
- AES-256-GCM encryption for configuration values
- Master key file creation and management
- Encryption-related tests and validation

## [0.1.0] - 2025-01-14

### Added

- Initial release of FinFocus Core CLI
- Projected cost calculation from Pulumi plans
- Actual cost queries with time ranges and filtering
- Cross-provider cost aggregation
- Plugin-based architecture for extensibility
- Configuration management system
- Multiple output formats (table, JSON, NDJSON)
- Resource grouping and filtering capabilities
- Comprehensive testing framework
