<p align="center"><a href="README.md">English</a> · <strong>हिन्दी</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic का आइकन: काले रंग पर दो गुलाबी छल्ले और एक प्ले बटन" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>YouTube Music और Spotify के लिए एक ही प्लेयर।</strong></p>

<p align="center">macOS, Windows और Linux के लिए YouTube Music और Spotify क्लाइंट। YouTube Music बिना अकाउंट के चलता है। एक सर्च, एक क्यू, मिली-जुली playlist, सिंक होने वाले lyrics, पॉडकास्ट, आपकी अपनी संगीत फ़ाइलें, कास्टिंग, थीम और प्लगइन।</p>

<p align="center"><sub>यह <a href="README.md">अंग्रेज़ी README</a> का अनुवाद है। जहाँ दोनों में फ़र्क़ हो, वहाँ अंग्रेज़ी वाला ताज़ा और सही है।</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="वेबसाइट" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="नवीनतम रिलीज़" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="डाउनलोड" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go का संस्करण" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Awesome MyGo में सूचीबद्ध" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows और Linux" />
  <a href="#लाइसेंस"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT या GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://mixmusic.functionx.dev/">वेबसाइट</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">डाउनलोड</a> ·
  <a href="https://mixmusic.functionx.dev/docs/">दस्तावेज़</a> ·
  <a href="docs/features.md">हर सुविधा</a> ·
  <a href="CHANGELOG.md">बदलावों की सूची</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic अपने डिफ़ॉल्ट रूप में, Mix थीम के साथ Blend लेआउट, गहरे मंच पर सीधा खड़ा और आपकी ओर मुँह किए हुए: बीच में YOASOBI का アイドル चल रहा है, गुलाबी किनारी के साथ, उसके हर lyrics की पंक्ति के नीचे romaji और सामने मिनी प्लेयर; बाईं ओर एक सर्च और दाईं ओर लाइब्रेरी के mix" />
</p>

## MixMusic क्यों

अगर आपका संगीत YouTube Music और Spotify में बँटा है, तो आमतौर पर आपको
दो ऐप चाहिए होते हैं, यानी दो सर्च, दो क्यू और दो लाइब्रेरी। MixMusic दोनों
सेवाओं को एक ही प्लेयर में ले आता है:

- **एक सर्च।** यह YouTube Music और Spotify में एक साथ खोजता है, और हर
  नतीजे पर उसकी सेवा का निशान लगाता है।
- **एक क्यू और एक लाइब्रेरी।** Spotify का गाना YouTube Music के गाने के ठीक
  बाद चल सकता है, और एक playlist में दोनों के गाने रह सकते हैं, और आपके
  कंप्यूटर की संगीत फ़ाइलों के भी।
- **अकाउंट की ज़रूरत नहीं।** ऐप खोलते ही YouTube Music चलने लगता है। साइन इन
  तभी करें जब आपको अपनी लाइब्रेरी चाहिए। Spotify वैकल्पिक है।
- **छोटा प्रोग्राम।** MixMusic एक ही Go प्रोग्राम है जो अपनी विंडो GPU पर
  खुद बनाता है। इसके अंदर कोई ब्राउज़र नहीं है जिसे डाउनलोड करना पड़े या
  मेमोरी में रखना पड़े।
- **अपने हिसाब से बदलें।** चार लेआउट और चौदह थीम साथ आते हैं, समुदाय और
  थीम तथा प्लगइन जोड़ता है, और हर कमांड को आप अपनी कुंजी दे सकते हैं।
- **आपकी भाषा में।** विंडो अंग्रेज़ी के अलावा 66 भाषाओं में आती है, और
  अरबी, हिब्रू, फ़ारसी और उर्दू के लिए दाएँ से बाएँ बिछती है।
- **निजी।** MixMusic का अपना कोई अकाउंट, सर्वर, analytics या telemetry
  नहीं है।
- **मुफ़्त और ओपन सोर्स,** MIT लाइसेंस या GPL 3.0 के तहत।

## सुविधाएँ

MixMusic क्या करता है, हिस्से-हिस्से में, और ऐप में आप उसे कहाँ पाएँगे।
**Built in** का मतलब है कि वह अपने आप काम करता है। [Features](docs/features.md)
0.1.0 से 0.5.12 तक की सभी 363 सुविधाएँ गिनाता है, हर एक की एक पंक्ति, और
यह कि वह किस संस्करण में आई।

### खोजें और चलाएँ

- **दोनों सेवाओं के लिए एक सर्च।** YouTube Music और Spotify के नतीजे साथ-साथ
  दिखते हैं, हर एक पर उसकी सेवा का निशान होता है, और उनके साथ आपकी अपनी
  संगीत फ़ाइलें भी। YouTube Music के नतीजों में दिखता है कि हर गाना कितनी बार
  चला, ताकि मूल गाना कवर से अलग पहचाना जाए। *ऊपर की पट्टी*
- **हर तरह का नतीजा।** गाने, वीडियो, एल्बम, कलाकार, playlist और पॉडकास्ट,
  सबसे सटीक, सबसे ज़्यादा चले या सबसे नए के क्रम में, और खुद YouTube भी उन
  लाइव सेट और अपलोड के लिए जो म्यूज़िक कैटलॉग में नहीं हैं। सर्च बॉक्स में
  चिपकाया गया YouTube या Spotify का लिंक तुरंत चलता है। *सर्च*
- **एक क्यू।** दोनों सेवाओं के गाने मिलाएँ, उन सबमें शफ़ल और रिपीट करें, किसी
  गाने को अगले पर चलने के लिए आगे खिसकाएँ, और क्यू खत्म होने पर रेडियो को
  आगे चलने दें। रीस्टार्ट के बाद वह वैसी ही लौटती है जैसी आपने छोड़ी थी।
  *प्लेयर बार, क्यू*
- **हर चीज़ का एक पेज।** एल्बम, कलाकार और playlist, Play और Shuffle के साथ,
  कलाकार के सारे गाने, और जो चल रहा है उससे जुड़ा संगीत। *सर्च, होम*
- **जो नहीं चाहिए उसे छोड़ें।** गानों को डिसलाइक करें ताकि रेडियो उन्हें छोड़ दें,
  AI से बने गाने और वीडियो के वे हिस्से छोड़ें जिन्हें SponsorBlock चिह्नित
  करता है, और explicit सामग्री छिपाएँ या साफ़ संस्करण चुनें। *प्लेयर बार,
  Settings › Playback*
- **स्लीप टाइमर और रफ़्तार।** कुछ समय, एक गाने या पूरी क्यू के बाद रुकें,
  धीरे-धीरे आवाज़ घटाते हुए, या रात में अपने आप, और गानों व पॉडकास्ट को
  उनकी अपनी पिच पर तेज़ या धीमा चलाएँ। *प्लेयर बार*
- **म्यूज़िक वीडियो और चलते कवर।** गाने के कवर की जगह उसका म्यूज़िक वीडियो,
  और कुछ एल्बमों के छोटे दोहराए जाने वाले वीडियो। *अभी चल रहा है,
  Settings › Playback*
- **चुनें, साझा करें और करीब से देखें।** कई गानों को चुनकर एक साथ क्यू में
  डालें, जोड़ें या डाउनलोड करें, किसी गाने या एल्बम को इमेज कार्ड के रूप में
  साझा करें, और देखें कि कोई गाना कहाँ से आता है और क्यों नहीं चल सका।
  *गाने का मेन्यू*

### होम और Explore

- **होम,** जिसमें आपकी शेल्फ़, Jump back in और आपके फ़ॉलो किए कलाकारों की नई
  रिलीज़ हैं, जैसे आप सजाएँ। *होम*
- **Explore:** नई रिलीज़, चार्ट, मूड और जॉनर। *Explore*

### Lyrics

- **सिंक होने वाले lyrics** जो गाने के साथ चलते हैं, जहाँ स्रोत में हो वहाँ
  शब्द-दर-शब्द, फ़ुल स्क्रीन में भी, या चाहें तो सादा टेक्स्ट। *Lyrics पैनल,
  अभी चल रहा है*
- **अनुवाद और लैटिन अक्षर:** हर पंक्ति के नीचे अनुवाद, जापानी के लिए romaji, और
  कोरियाई, चीनी, रूसी, अरबी और कई दूसरी लिपियों के लिए लैटिन अक्षर।
  *Lyrics पैनल*
- **स्रोत आपकी पसंद के:** Better Lyrics, Unison, LRCLIB, YouTube Music और
  YouTube के कैप्शन, आपके तय क्रम में, और दूसरा स्रोत, सर्च, आपके अपने
  बदलाव और टाइमिंग बस एक क्लिक दूर। *Settings › Playback › Lyrics,
  Lyrics पैनल*
- **Lyrics में खोजें,** उनकी पंक्तियाँ कार्ड के रूप में साझा करें, और अपनी गाना
  फ़ाइलों में रखे lyrics पढ़ें। *Lyrics पैनल*

### पॉडकास्ट

- **YouTube Music और Spotify के पॉडकास्ट।** शो खोजें और फ़ॉलो करें, हर एपिसोड
  वहीं से शुरू करें जहाँ छोड़ा था, एपिसोड को सुना हुआ चिह्नित करें, उनके
  चैप्टर और ट्रांसक्रिप्ट पढ़ें, और उन्हें गानों से अलग रफ़्तार दें।
  *सर्च, शो का पेज, Library › Podcasts*

### आपकी लाइब्रेरी

- **Mix:** ऐसी playlist जिनमें हर सेवा के गाने रह सकते हैं, आपके अपने क्रम में,
  खोजी और सॉर्ट की जा सकने वाली, आपके अपने कवर के साथ। *Library › Mixes,
  साइडबार*
- **पसंद किए गए गाने** दोनों सेवाओं से, एक ही सूची में, और आपकी YouTube Music
  और Spotify की playlist, एल्बम और कलाकार, जिन्हें आप सेव कर सकते हैं, बना
  सकते हैं, साझा कर सकते हैं और मिलकर बना सकते हैं। लाइक और सेव सेवा तक भी
  पहुँचते हैं। *Library*
- **पिन, फ़ोल्डर और टैग** बड़ी लाइब्रेरी को सलीके से रखते हैं, और जो आपको वहाँ
  नहीं चाहिए उसे छिपाया जा सकता है। *साइडबार, टाइल का मेन्यू*
- **कलाकारों को फ़ॉलो करें,** अकाउंट के साथ या बिना, और उनकी नई रिलीज़ देखें।
  *कलाकार का पेज, Library › Artists*
- **हिस्ट्री, Stats और Your year:** दिन-दर-दिन मिनट, आपके सबसे ज़्यादा सुने गाने
  और कलाकार, और उनके साझा करने लायक कार्ड। *Library › History, Stats*
- **डाउनलोड** बिना नेटवर्क के चलाने के लिए, और आपके सुने गानों का ऑफ़लाइन कैश।
  *गाने का मेन्यू, Library › Downloads*

### इस कंप्यूटर का संगीत

- **आपकी अपनी संगीत फ़ाइलें** लाइब्रेरी और सर्च में, उन फ़ोल्डरों से जिन पर
  MixMusic नज़र रखता है: MP3, FLAC, WAV, M4A, Opus और Vorbis, उनके टैग और
  कवर के साथ। *Settings › Playback › Music on this computer*
- **उनके टैग बदलें,** एक गाने की कॉपियाँ इकट्ठी करें, और फ़ाइलें खोलें या
  गिराकर चलाएँ। *गाने का मेन्यू, विंडो पर गिराएँ*

### अकाउंट और सेवाएँ

- **YouTube Music।** Google के अपने पेज पर साइन इन करें, अपने ब्राउज़र से या
  किसी ब्राउज़र के सेशन से, अपने अकाउंट से या उसके किसी ब्रांड चैनल से, अपने
  होम फ़ीड, पसंद किए गाने, playlist, एल्बम और कलाकारों के लिए। आप जो सुनते
  हैं वह आपकी YouTube Music हिस्ट्री में जुड़ता है, जिससे उसके सुझाव बनते
  हैं। *Settings › Accounts*
- **Last.fm:** आपका सुना हर गाना, आधे तक या चार मिनट तक सुनने पर, आपकी
  प्रोफ़ाइल में जुड़ता है, और ऑफ़लाइन हुए scrobble ऑनलाइन लौटते ही भेज दिए
  जाते हैं। *Settings › Accounts*
- **ListenBrainz:** यही, Last.fm के साथ या उसके बिना, ListenBrainz पर या आपके
  अपने किसी सर्वर पर, जैसे Maloja। लाइक किए गाने दोनों में से किसी पर भी love के
  रूप में जा सकते हैं। *Settings › Accounts*
- **Libre.fm:** यही, Libre.fm पर या किसी दूसरे GNU FM सर्वर पर, बिना कोई API
  अकाउंट बनाए। *Settings › Accounts*
- **Discord:** आपकी प्रोफ़ाइल दिखाती है कि क्या चल रहा है, उसकी आर्टवर्क और
  बचे हुए समय के साथ, जैसा आप चुनें। *Settings › Desktop*

### Spotify, तीन तरीकों से

- **सीधे MixMusic में,** Spotify की क्वालिटी पर, बिना Spotify ऐप खोले
  (Premium, Windows और macOS पर)।
- **Spotify Connect पर:** आपका Spotify ऐप, आपका फ़ोन या कोई स्पीकर, lossless में
  जब Spotify ऐप में वह चालू हो।
- **YouTube Music से:** MixMusic Spotify के गाने की रिकॉर्डिंग YouTube Music
  पर चलाता है, जिसके लिए Premium नहीं चाहिए।

Settings › Playback › Spotify tracks में चुनें।

### आवाज़

- **दस बैंड का इक्वलाइज़र** 23 प्रीसेट और आपके अपने प्रीसेट के साथ, बास, मिड और
  ट्रेबल का एक सरल मोड, और AutoEq से 8,800 से ज़्यादा हेडफ़ोन का सुधार।
  *Settings › Playback › Equalizer*
- **क्रॉसफ़ेड, बिना अंतराल वाले एल्बम, एक जैसी लाउडनेस और वॉल्यूम लेवलिंग।**
  *Settings › Playback › Sound*
- **रीवर्ब, इको, क्रॉसफ़ीड, बैलेंस और स्टीरियो की चौड़ाई।** *Settings ›
  Playback*
- **5.1 सराउंड** जहाँ गाने में हो, और आपकी पसंद का आउटपुट डिवाइस। *प्लेयर बार,
  Settings › Playback › Sound*
- **एक विज़ुअलाइज़र,** MixMusic के अपने ऑडियो इंजन से बना। *अभी चल रहा है*

### कास्टिंग, दोस्त और दूसरे कंप्यूटर

- **कास्ट करें** Chromecast, Google Home और Nest स्पीकर और उनके ग्रुप, DLNA और
  UPnP रेंडरर और FCast रिसीवर पर, अपनी संगीत फ़ाइलें भी। *प्लेयर बार,
  मिनी प्लेयर*
- **दोस्तों के साथ सुनें:** एक कमरा चलाएँ जो आपके दोस्तों के कंप्यूटर पर एक साथ
  चलता है, आपके नेटवर्क पर या कहीं से भी आमंत्रण लिंक से। *प्लेयर बार का
  कास्ट मेन्यू*
- **रिमोट कंट्रोल:** अपने नेटवर्क के किसी दूसरे कंप्यूटर पर MixMusic को चलाएँ,
  और जो चल रहा है उसे उनके बीच ले जाएँ। *प्लेयर बार का कास्ट मेन्यू,
  Settings › Desktop*
- **AI असिस्टेंट** आपके कंप्यूटर पर बता सकते हैं कि क्या चल रहा है, सर्च कर सकते
  हैं, चला सकते हैं और स्लीप टाइमर लगा सकते हैं, Model Context Protocol के
  ज़रिए। *Settings › Desktop › AI assistants*

### आपके डेस्कटॉप पर

- **एक मिनी प्लेयर** जो आपकी बाकी विंडो के ऊपर बना रहता है, अपने पैनल में क्यू
  और lyrics के साथ। *प्लेयर बार, <kbd>Ctrl</kbd> <kbd>M</kbd>*
- **मीडिया कुंजियाँ** और सिस्टम के मीडिया कंट्रोल, Windows, macOS और Linux पर।
  *Built in*
- **ग्लोबल शॉर्टकट** आपके अपने, प्ले, पॉज़, स्किप, वॉल्यूम बदलने, म्यूट, चल रहे
  गाने को लाइक या डिसलाइक करने या MixMusic दिखाने के लिए, चाहे कोई भी विंडो
  सामने हो। *Settings › Shortcuts*
- **प्लेयर के बटन** Windows में टास्कबार थंबनेल पर, और MacBook के नॉच के आसपास
  एक प्लेयर। *Built in, Settings › Desktop*
- **ट्रे कंट्रोल,** नए गानों की सूचनाएँ, हमेशा ऊपर रखना और लॉगिन पर शुरू होना।
  *Settings › Desktop*
- **`mixmusic://` लिंक** और `--next` जैसे फ़्लैग, Stream Deck की कुंजी या
  स्क्रिप्ट के लिए। *कमांड लाइन*
- **ओवरले के लिए अभी चल रहा है:** OBS ब्राउज़र सोर्स के लिए चल रहे गाने का
  कार्ड, और आपके अपने पेज के लिए उसका डेटा। *Settings › Desktop*

### इसे अपना बनाएँ

- **लेआउट।** Blend, यानी डिफ़ॉल्ट, में बड़े कवर की घनी पंक्तियों पर गोल आकार
  और उछलती हुई गति है। Expressive, Material 3 Expressive की तर्ज़ पर, चंचल है,
  टोनल फ़िल, पिल बटन और तैरते प्लेयर बार के साथ। Crate संयमित है, ताकि कवर
  ही काम करें, और Tiles MixMusic का पहला रूप है, जिसमें सपाट टाइल और
  विशाल छोटे अक्षरों के शब्द हैं। *Settings › Appearance*
- **थीम।** चौदह बिल्ट-इन, हर एक किसी भी लेआउट में, ट्यून किए जा सकने वाले
  इफ़ेक्ट के साथ, लाइट मोड के लिए एक थीम और डार्क मोड के लिए एक, और आपका
  अपना या चल रही आर्टवर्क से लिया गया एक्सेंट रंग। Material और Material
  Dark हर रंग कवर से बनाते हैं। *Settings › Appearance*
- **समुदाय की थीम और प्लगइन,** मार्केटप्लेस में बस एक क्लिक दूर।
  *मार्केटप्लेस*
- **प्लगइन** एक टाइप्ड API के ज़रिए पेज, कमांड, गाने की क्रियाएँ और विजेट
  जोड़ते हैं, जो आपकी थीम में बने दिखते हैं। *Settings › Plugins*
- **कीबोर्ड पहले।** हर आम काम के लिए शॉर्टकट, किसी भी कमांड के लिए आपकी अपनी
  कुंजी, प्लगइन की कमांड समेत, किसी भी ऐप से संगीत चलाने वाली कुंजियाँ, और
  बाकी सब के लिए कमांड पैलेट (<kbd>Ctrl</kbd> <kbd>K</kbd>)।
- **आपकी भाषा।** अंग्रेज़ी के अलावा 66 भाषाएँ, जहाँ भाषा दाएँ से बाएँ पढ़ी जाती है
  वहाँ दाएँ से बाएँ, और तारीख़ें व संख्याएँ वैसी जैसी आपका क्षेत्र लिखता है।
  *Settings › Appearance › Language*
- **स्क्रीन रीडर और कीबोर्ड के लिए।** स्क्रीन रीडर के लिए नाम और घोषणाएँ, हर
  कंट्रोल कीबोर्ड की पहुँच में, टेक्स्ट का आकार, ज़ूम और कम गति। *Built in,
  Settings › Appearance*
- **एक सेटिंग फ़ाइल।** आपकी सेटिंग आपके डेटा फ़ोल्डर में `settings.json` है
  (Windows पर `%AppData%\MixMusic`, macOS पर
  `~/Library/Application Support/MixMusic`, Linux पर `~/.config/MixMusic`),
  जिसे आप अपनी dotfiles के साथ रख सकते हैं। MixMusic चलते समय इसे सेव करें,
  और यह तुरंत लागू हो जाती है। *Settings › Desktop › Settings file*

### बैकअप, इंपोर्ट और एक्सपोर्ट

- **बैकअप लें और बहाल करें** अपनी लाइब्रेरी, सेटिंग और ऐड-ऑन, हाथ से या अपने
  आप, इसी कंप्यूटर पर या किसी दूसरे पर। *Settings › Desktop › Backups*
- **Playlist इंपोर्ट करें** फ़ाइलों से और अपने Spotify और Google डेटा से, और सूचियाँ
  व अपनी लाइब्रेरी ऐसी फ़ाइलों में एक्सपोर्ट करें जिन्हें दूसरे प्लेयर पढ़ते हैं।
  *Library › Mixes, पेज का More मेन्यू*

### अपडेट

- MixMusic शुरू होने पर और हर कुछ घंटों में नई रिलीज़ देखता है, उसका सिग्नेचर
  जाँचता है, और जब आप कहें तब उसे इंस्टॉल करता है, या आप चुनें तो अपने आप।
  *Settings › About*

## एक नज़र

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend लेआउट में MixMusic के पेज, गहरे मंच पर सीधे खड़े और आपकी ओर मुँह किए हुए: बीच में YOASOBI का アイドル चल रहा है, गुलाबी किनारी के साथ, उसके lyrics की हर पंक्ति के नीचे romaji और अंग्रेज़ी अनुवाद; बाईं ओर एल्बम THE BOOK 3 और दाईं ओर फ़ॉलो किए गए एक पॉडकास्ट का पेज" />
</p>

<p align="center"><sub><b>सुनें।</b> बीच में YOASOBI का アイドル चल रहा है: हर पंक्ति गाते समय रोशन होती है, उसके नीचे romaji और अनुवाद के साथ। बाईं ओर एक एल्बम; दाईं ओर आपका फ़ॉलो किया हुआ एक पॉडकास्ट।</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic के और हिस्से, गहरे मंच पर सीधे खड़े और आपकी ओर मुँह किए हुए: बीच में Settings › Appearance (सेटिंग › दिखावट) चारों लेआउट के साथ, गुलाबी किनारी में; बाईं ओर चल रहे कवर से रंगी Material थीम और दाईं ओर मार्केटप्लेस की समुदाय की थीम" />
</p>

<p align="center"><sub><b>इसे अपना बनाएँ।</b> बीच में चारों लेआउट: Blend, यानी डिफ़ॉल्ट, Expressive, Crate और Tiles, थीम के नीचे। बाईं ओर एक Material थीम जो चल रहे कवर से अपने रंग लेती है। दाईं ओर मार्केटप्लेस में समुदाय की थीम।</sub></p>

## दस्तावेज़

गाइड वेबसाइट पर हैं,
[mixmusic.functionx.dev/docs](https://mixmusic.functionx.dev/docs/) पर, और
[fzn0x.github.io/mixmusic/docs](https://fzn0x.github.io/mixmusic/docs/) पर भी,
जहाँ सर्च किसी भी गाइड को ढूँढ देता है, और यहाँ [`docs/`](docs) में भी।

- **शुरुआत:** [Getting started](docs/getting-started.md),
  [Features](docs/features.md), [Accounts](docs/accounts.md),
  [Spotify](docs/spotify.md), [Troubleshooting](docs/troubleshooting.md)
- **MixMusic का इस्तेमाल:** [Playing music](docs/playing-music.md),
  [Home](docs/home.md), [Your library](docs/library.md),
  [Import and export playlists](docs/import-export.md),
  [Music on this computer](docs/local-music.md), [Lyrics](docs/lyrics.md),
  [Podcasts](docs/podcasts.md), [Look and feel](docs/look-and-feel.md),
  [On the desktop](docs/desktop.md),
  [Overlays for streams](docs/stream-overlay.md),
  [Casting](docs/casting.md), [Remote control](docs/remote-control.md),
  [Listen with friends](docs/listen-with-friends.md),
  [AI assistants](docs/ai-assistants.md),
  [Marketplace](docs/marketplace.md), [Settings](docs/settings.md)
- **यह कैसे काम करता है:** [How MixMusic works](docs/how-it-works.md),
  [The audio engine](docs/audio-engine.md),
  [Where the audio streams come from](docs/streams.md),
  [How Spotify tracks play](docs/spotify-playback.md),
  [Your data and privacy](docs/data-and-privacy.md)
- **अपना बनाएँ:** [Themes](docs/themes.md), [Plugins](docs/plugins.md)
- **प्रोजेक्ट:** [Credits](docs/credits.md), [License](docs/license.md),
  [Architecture](docs/architecture/README.md),
  [Contributing](CONTRIBUTING.md), [Code of conduct](CODE_OF_CONDUCT.md),
  [Security](SECURITY.md)

## इंस्टॉल करें

[नवीनतम रिलीज़](https://github.com/fzn0x/mixmusic/releases/latest) से अपने सिस्टम
की फ़ाइल डाउनलोड करें:

| सिस्टम | डाउनलोड |
|---|---|
| macOS 12 और उसके बाद (Apple silicon और Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 और 11 (x64, Arm), इंस्टॉलर | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, पोर्टेबल, बिना इंस्टॉल किए | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu और उन पर आधारित | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE और उन पर आधारित | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux और उस पर आधारित (जल्द आ रहा है) | AUR पर [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin): `yay -S mixmusic-bin` |
| कोई भी Linux, एक फ़ाइल | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| कोई भी Linux, सैंडबॉक्स में (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| कोई भी Linux, आपके होम फ़ोल्डर में | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), `install.sh` के साथ |

`<arch>` x64 के लिए `amd64` और Arm के लिए `arm64` है। हर रिलीज़ में
`SHA256SUMS` दिया गया है, जिससे आप डाउनलोड जाँच सकते हैं। `update-…`,
`….delta`, `mixmusic-<version>-darwin-universal.tar.gz` या
`-windows-<arch>.tar.gz` नाम की फ़ाइलें आपको नहीं चाहिए: इंस्टॉल किए गए ऐप
इन्हीं से अपडेट होते हैं।

इंस्टॉलर, पोर्टेबल zip और Linux आर्काइव खुद को अपडेट करते हैं। `.deb`, `.rpm`,
AppImage और Flatpak अपनी फ़ाइलें खुद नहीं बदल सकते, इसलिए नया संस्करण आने पर
MixMusic आपको बताता है और उसे आपके लिए डाउनलोड कर देता है। AUR का
`mixmusic-bin` आपके बाकी सिस्टम के साथ अपडेट होता है। पोर्टेबल zip के लिए, उसे
कहीं भी खोलें और `MixMusic.exe` चलाएँ। AppImage के लिए, उसे executable बनाएँ
(`chmod +x`) और चलाएँ। Flatpak के लिए, उसे अपने सॉफ़्टवेयर सेंटर से खोलें, या
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak` चलाएँ; पहली बार
Flatpak, Flathub से GNOME runtime लाता है।

Mac पर [Homebrew](https://brew.sh) भी MixMusic इंस्टॉल करता है, और
`brew upgrade` उसे अपडेट करता है:

```sh
brew install --cask fzn0x/tap/mixmusic
```

[Getting started](docs/getting-started.md) हर सिस्टम पर इंस्टॉल और पहली बार
चलाने का तरीका क़दम-दर-क़दम बताता है।

### पहली बार चलाना

- **macOS:** ऐप को अभी Apple से notarize नहीं कराया गया है, इसलिए पहली बार
  खोलने पर macOS कहता है कि वह इसे जाँच नहीं सकता। इसे फिर भी खोलें: System
  Settings › Privacy & Security › Open Anyway (सिस्टम सेटिंग › गोपनीयता और
  सुरक्षा › फिर भी खोलें) से, या Terminal में यह एक बार चलाएँ:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** इंस्टॉलर पर अभी साइन नहीं है, इसलिए SmartScreen चेतावनी दे सकता
  है: More info चुनें, फिर Run anyway। YouTube Music में साइन इन करने और
  MixMusic में Spotify चलाने के लिए WebView2 Runtime चाहिए, जो Windows 11 में
  पहले से है।
- **Linux:** MixMusic को GTK 3 चाहिए, और PulseAudio (PipeWire भी उसे चलाता है)
  या ALSA। YouTube Music में साइन इन करने के लिए WebKitGTK 4.1 चाहिए, ट्रे आइकन
  के लिए `libayatana-appindicator3`, और आपके साइन-इन को सिस्टम की keyring में रखने
  के लिए `libsecret`। `.deb` और `.rpm` इन्हें इंस्टॉल कर देते हैं, और Flatpak
  इन्हें साथ लाता है; AppImage और आर्काइव के लिए:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```

  MixMusic के साथ चीनी, जापानी और कोरियाई अक्षरों का कोई फ़ॉन्ट नहीं आता, और
  Linux पर जिस अक्षर का फ़ॉन्ट इंस्टॉल न हो वह खाली जगह दिखता है। `.deb` और
  `.rpm` एक फ़ॉन्ट माँगते हैं; बाकी के साथ, खुद एक इंस्टॉल करें और MixMusic को
  फिर शुरू करें:

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
  ```
- **हर सिस्टम:** MixMusic YouTube Music के ज़्यादातर गानों का ऑडियो खुद ढूँढ लेता
  है। बाकी के लिए, जैसे बच्चों के लिए बने गाने, यह
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) का इस्तेमाल करता है, जिसे यह
  Settings › Playback (सेटिंग › प्लेबैक) से आपके लिए इंस्टॉल करता है और अपडेट
  रखता है। yt-dlp को एक JavaScript runtime चाहिए: [Deno](https://deno.com),
  Node.js, Bun या QuickJS। Flatpak में Node.js है।

### Flatpak

Flatpak, MixMusic को एक सैंडबॉक्स में चलाता है, जिसे नेटवर्क, साउंड सर्वर,
डिस्प्ले और GPU दिखते हैं, और आपके होम फ़ोल्डर में से कुछ नहीं, सिवाय Flathub से
इंस्टॉल किए ब्राउज़रों की प्रोफ़ाइल के, सिर्फ़ पढ़ने के लिए, जिनसे YouTube Music
का साइन-इन इंपोर्ट किया जा सकता है। यह x64 और Arm64 दोनों के लिए आता है, जैसे
Raspberry Pi 5, Asahi Linux और Snapdragon लैपटॉप। MixMusic अभी Flathub पर नहीं
है: रिलीज़ का `.flatpak` इंस्टॉल करें।

Flatpak अपनी सेटिंग और लाइब्रेरी `~/.var/app/io.github.fzn0x.mixmusic` में रखता
है, जो `.deb`, `.rpm`, AppImage या आर्काइव वाली कॉपी से अलग है, और दोनों एक साथ
चल सकती हैं। किसी कॉपी की सेटिंग और लाइब्रेरी लाने के लिए, उसे Settings ›
Desktop › Backups में बैकअप करें और फ़ाइल को Flatpak में बहाल करें। फ़ाइलें
हाथ से कॉपी करनी हों, तो दोनों बंद करें, फिर `~/.config/MixMusic` से
`settings.json`, `library.json`, `plays.jsonl`, `local-copies.json`,
`queue.json`, `positions.json` और `addons` व `mix-covers` फ़ोल्डर
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` में कॉपी करें। पहले दोनों
बंद करें: चलता हुआ MixMusic `library.json` और ज़्यादातर दूसरी फ़ाइलें मेमोरी में
रखता है और अगली बार सहेजते समय कॉपी की गई फ़ाइल को अधिलेखित कर देता है। फिर अपने
अकाउंट में दोबारा साइन इन करें: साइन-इन दूसरी कॉपी की keyring में ही रहते हैं।
[डेटा और गोपनीयता का पेज](docs/data-and-privacy.md#the-data-folder) बताता है कि हर
फ़ाइल में क्या है।

Flatpak में, लॉगिन पर शुरू होना आपके डेस्कटॉप से होकर जाता है, जो पहले आपसे
पूछ सकता है, और आपका अपना yt-dlp `~/.var/app/io.github.fzn0x.mixmusic` में
होना चाहिए, क्योंकि सैंडबॉक्स को कोई और फ़ोल्डर नहीं दिखता। जो Discord ऐप
Flatpak नहीं है, वह चल रहा गाना तभी दिखाता है जब MixMusic के शुरू होते समय वह
चल रहा हो, वरना उसे दोबारा शुरू करने तक; Flatpak वाला Discord दोनों तरह से
काम करता है। Flatpak नया है: किसी [issue](https://github.com/fzn0x/mixmusic/issues)
में हमें बताएँ कि यह आपके डेस्कटॉप पर कैसा चलता है।

### अकाउंट (सभी वैकल्पिक)

- **YouTube Music:** Settings › Accounts (सेटिंग › अकाउंट) में Sign in to
  YouTube Music चुनें और खुलने वाली विंडो में Google में साइन इन करें। आप जो
  टाइप करते हैं वह सिर्फ़ Google को जाता है। MixMusic सेशन को एन्क्रिप्ट करके
  रखता है, और जहाँ keyring है वहाँ उसकी कुंजी आपके सिस्टम की keyring में रखता
  है। जहाँ वह विंडो खुल नहीं सकती, या Google उसे मना कर देता है, वहाँ अपने
  ब्राउज़र से साइन इन करें, जिसे MixMusic अपनी एक नई प्रोफ़ाइल के साथ खोलता है
  और आपके अंदर आते ही मिटा देता है; या उस ब्राउज़र का सेशन इंपोर्ट करें जिसमें
  आपने साइन इन किया है (MixMusic उसे अपने yt-dlp से पढ़ता है, और सिर्फ़ YouTube
  की कुकी रखता है); या साइन इन किए हुए music.youtube.com टैब का Cookie हेडर
  पेस्ट करें।
- **Spotify:** Spotify एक ऐप को पाँच उपयोगकर्ताओं तक ही चलाने देता है, और सिर्फ़
  तब तक जब तक उसके मालिक के पास Premium है, इसलिए आप अपने ही बनाए Spotify ऐप
  से साइन इन करते हैं। इसमें दो मिनट लगते हैं: [docs/spotify.md](docs/spotify.md)
  आपको पूरा तरीका बताता है।
- **Last.fm:** [last.fm/api/account/create](https://www.last.fm/api/account/create)
  पर एक API अकाउंट बनाएँ (कोई भी नाम, कोई callback URL नहीं), उसकी API key और
  shared secret को Settings › Accounts में पेस्ट करें, और अपने ब्राउज़र में
  MixMusic को मंज़ूरी दें।
- **ListenBrainz:** अपनी [ListenBrainz सेटिंग](https://listenbrainz.org/settings/)
  का, या अपने सर्वर का, user token Settings › Accounts में पेस्ट करें।
- **Libre.fm:** Settings › Accounts में जोड़ें, और अपने ब्राउज़र में Libre.fm के
  पेज पर, या किसी दूसरे GNU FM सर्वर के पेज पर, MixMusic को मंज़ूरी दें।
- **Discord:** जब तक Discord ऐप चल रहा है, आपकी प्रोफ़ाइल दिखाती है कि आप क्या
  सुन रहे हैं। इसे Settings › Desktop में बंद करें।

[Accounts](docs/accounts.md) हर साइन-इन क़दम-दर-क़दम बताता है।

### ज्ञात सीमाएँ

- **Arm पर Windows** x64 से कम जाँचा गया है, और पोर्टेबल zip `mixmusic://`
  लिंक रजिस्टर नहीं करता।
- **डेवलपमेंट मोड वाले Spotify ऐप,** जैसा वह जिससे आप साइन इन करते हैं, हर तरह
  के दस सर्च नतीजे देते हैं, और जिन playlist के आप मालिक नहीं हैं उनके गाने
  छिपे रहते हैं।
- **Linux पर,** Spotify डिफ़ॉल्ट रूप से Spotify Connect से चलता है, क्योंकि
  WebKitGTK Spotify का सुरक्षित ऑडियो नहीं चला सकता।
- **YouTube** अपने इंटरफ़ेस बदल सकता है और सर्च, प्लेबैक या साइन-इन तब तक तोड़
  सकता है जब तक MixMusic या yt-dlp उसके साथ न चल पाए।

कुछ काम नहीं कर रहा? [Troubleshooting](docs/troubleshooting.md) में आम
समस्याएँ और ज्ञात दिक्कतें हैं।

## सोर्स से बनाएँ

आपको [Go](https://go.dev/dl/) 1.27.3 या उसके बाद का, [Bun](https://bun.sh) 1.4
या उसके बाद का, जो MyGo की कमांड लाइन चलाता है, और Git चाहिए। MyGo को C
कंपाइलर नहीं चाहिए, इसलिए कोई भी सिस्टम हर सिस्टम के लिए MixMusic बना सकता है।

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

नया टर्मिनल खोलें, फिर:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

साइन इन करने और Spotify चलाने के लिए WebView2 Runtime चाहिए, जो Windows 11 में
पहले से है। Windows 10 पर उसे
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/) से
इंस्टॉल करें।

### macOS

[Homebrew](https://brew.sh) के साथ:

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Go को [go.dev/dl](https://go.dev/dl/) से इंस्टॉल करें (डिस्ट्रीब्यूशन के पैकेज
अक्सर 1.27.3 से पुराने होते हैं), फिर Bun और वे लाइब्रेरी जिनके साथ MixMusic
चलता है:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

फिर:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

[Nix](https://nixos.org) के साथ, `nix develop` आपको वे टूल देता है जिनसे
रिलीज़ बनती हैं, उन संस्करणों में जिन्हें `flake.lock` पिन करता है: Go 1.27, Bun,
GoReleaser, jq, zip और Zola, और Linux पर NSIS, osslsigncode और binutils।
`nix build` MixMusic को Linux के लिए सोर्स से `result/` में बनाता है, और
`nix run github:fzn0x/mixmusic` उसे चलाता है।

### विकास करें

`bun run dev` ऐप बनाता है, उसे शुरू करता है, और आपके बदलाव करते ही उसे दोबारा
बनाता है। इसका डेटा इंस्टॉल किए गए MixMusic से अलग रहता है, `MixMusic Dev`
नाम से।

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

Nix शेल में, Linux के डाउनलोड वैसे ही बनाएँ जैसे रिलीज़ बनती हैं, यानी
`.github/scripts/build.sh linux/amd64,linux/arm64` से। वहाँ सादा `bun run build`
ऐसा Linux प्रोग्राम बनाता है जो Nix का अपना loader माँगता है, और वह सिर्फ़ NixOS
पर शुरू होता है। `go run ./tools/elfloader build/linux-amd64` किसी बिल्ड की
जाँच करता है।

[वेबसाइट](https://mixmusic.functionx.dev/) `site/` में है, जो
[Zola](https://www.getzola.org) से बनी है, और `docs/` की गाइड उसके `/docs/`
पेज हैं। `go run ./tools/sitedocs && zola --root site serve` उसका प्रीव्यू
दिखाता है। रिलीज़ Nix शेल में बनती हैं और [GoReleaser](https://goreleaser.com)
(`.goreleaser.yaml`) से प्रकाशित होती हैं।

[CONTRIBUTING.md](CONTRIBUTING.md) में पुल रिक्वेस्ट से पहले चलाने की जाँचें हैं,
और [docs/architecture](docs/architecture/README.md) बताता है कि हिस्से आपस में
कैसे जुड़ते हैं, हर एक के लिए एक पेज।

### संरचना

ऐप रूट पैकेज है, और उसके नीचे `internal/` के पैकेज हैं। `internal/` के बाहर के
पैकेज किसी के भी import करने के लिए हैं, और
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic) पर documented हैं।

```
*.go                 the app and its windows: the pages, the player bar, the
                     mini player, the tray, media keys, deep links
fonts/               the typefaces the windows draw with
locales/             the app's words, in English and 66 other languages
internal/app         the services the windows call, and the desktop integration
internal/player      the queue, played by the audio engine or on Spotify's devices
internal/audio       MixMusic's audio engine, and the system's audio output
internal/stream      audio streams, from YouTube or yt-dlp, opened for the engine
internal/youtube     YouTube Music's catalog, and your account there
internal/spotify     the Spotify Web API, its sign-in and Spotify Connect
internal/match       Spotify tracks matched to their YouTube Music recordings
internal/library     liked and disliked songs, mixes, history and stats
internal/localfiles  the music on this computer; tags and tagwrite, its tags
internal/romaji      Japanese lyrics in romaji, kanji read by kagome;
                     romanize, the Latin letters of other scripts
internal/cast        casting to Chromecasts; upnp, to DLNA renderers; fcast,
                     to FCast receivers; dnssd finds them on the network
internal/room        Listen with friends; remote, remote control; portmap,
                     the router's forwarded port
internal/mcp         AI assistants, over the Model Context Protocol
internal/lastfm      scrobbling to Last.fm; listenbrainz and librefm, to
                     theirs; scrobble, which plays count
internal/discord     what plays, on your Discord profile
internal/plugins     plugins, each in a JavaScript engine of its own
internal/addons      themes and plugins, and the community registry
internal/backup      backups and restores; playlistio and export, the files
                     of lists and tracks in and out
internal/diag        the log, without secrets, and the reports of crashes
internal/settings    your preferences and their defaults
builtin/themes       the themes MixMusic ships with
public/              the pages MyGo embeds: Spotify's player, on Windows and
                     macOS, and the picture of music videos
sdk/                 the types of the plugin API
community/           community themes and plugins, and their registry
docs/                the guides, the website's /docs/ pages
site/                the website, built with Zola
tools/               the commands that build and check the repository's parts
packaging/, nix/     the Flatpak, Flathub, AUR and Homebrew packages, and Nix
changelog.d/         the changelog entries of what isn't released yet
```

[docs/architecture](docs/architecture/README.md) `internal/` के बाकी पैकेजों के
बारे में बताता है, ऐप के हर हिस्से के लिए एक पेज।

जो पैकेज आप import कर सकते हैं:

```
opus/                an Opus decoder in pure Go, a port of libopus's
aac/                 an AAC-LC decoder in pure Go
ac3/                 an AC-3 and E-AC-3 decoder in pure Go, written from ATSC A/52
flac/                a FLAC decoder in pure Go, as RFC 9639 has it
ogg/                 the packets of Ogg files, as RFC 3533 has them
webm/                the audio of WebM files: packets, times and seeking
mp4/                 the audio of MP4 files: packets, times and seeking
resample/            sample rate conversion, with Kaiser-windowed sincs
lyrics/              synced lyrics from Better Lyrics, Unison and LRCLIB, and translations
brand/               MixMusic's logo, for code that draws it
```

## समुदाय

कोई भी मार्केटप्लेस से समुदाय की थीम और प्लगइन इंस्टॉल कर सकता है, जो साइडबार में
स्टोर का आइकन है, और आप अपनी भी जोड़ सकते हैं।

### थीम बनाएँ

थीम एक `theme.json` फ़ाइल है जो रंग, फ़ॉन्ट और कोने तय करती है, जो उपयोगकर्ता के
चुने लेआउट में दिखते हैं। [docs/themes.md](docs/themes.md) हर टोकन गिनाता है और
एक पूरी थीम दिखाता है। अपनी थीम आज़माने के लिए, उसका फ़ोल्डर थीम फ़ोल्डर में रखें
(Settings › Appearance › open themes folder (सेटिंग › दिखावट › थीम फ़ोल्डर खोलें))
और उसे Settings › Appearance में चुनें।

### प्लगइन बनाएँ

प्लगइन एक छोटा मॉड्यूल है, जो TypeScript या JavaScript में लिखा जाता है और पेज,
कमांड, गाने की क्रियाएँ और विजेट जोड़ता है। [docs/plugins.md](docs/plugins.md)
API को documented करता है, आपके एडिटर के लिए उसके [types](sdk/index.d.ts) के साथ।
समुदाय के [sleep timer](community/plugins/sleep-timer) और
[spectrum](community/plugins/spectrum) चलते-फिरते उदाहरण हैं।

### इसे साझा करें

1. इस रिपॉज़िटरी को fork करें।
2. एक फ़ोल्डर जोड़ें: थीम के लिए `community/themes/<id>/` उसकी `theme.json` के
   साथ, या प्लगइन के लिए `community/plugins/<id>/` उसकी `plugin.json` और उसकी
   `index.ts` के साथ।
3. `bun install`, `bun run build:plugins`, `go run ./tools/registry` और
   `go run ./tools/registry -check` चलाएँ।
4. एक स्क्रीनशॉट के साथ pull request खोलें।

मर्ज होने के बाद, CI उसे
[registry branch](https://github.com/fzn0x/mixmusic/tree/registry) पर प्रकाशित
करता है, और वह सबके मार्केटप्लेस में दिखने लगती है।
[community/README.md](community/README.md) में ब्योरा है।

## गोपनीयता

MixMusic का अपना कोई अकाउंट, सर्वर, analytics या telemetry नहीं है। यह इनसे बात
करता है:

- **YouTube और Google:** YouTube Music का कैटलॉग, उसकी स्ट्रीम और हर गाना कितना
  तेज़ है, साइन आउट रहते हुए माँगा गया, और आर्टवर्क; कुछ स्ट्रीम yt-dlp के
  ज़रिए। साइन इन रहने पर, वहाँ की आपकी लाइब्रेरी, और, जब तक आप Settings ›
  Accounts (सेटिंग › अकाउंट) में इसे बंद न करें, आपके सुने गाने, जो आपकी
  YouTube Music हिस्ट्री में जुड़ते हैं। खुद YouTube, साइन आउट और बिना कुकी के,
  सर्च के YouTube विकल्प, AI से बने गानों की जाँच, lyrics और ट्रांसक्रिप्ट के
  रूप में कैप्शन, और म्यूज़िक वीडियो से समयबद्ध lyrics के लिए।
- **Spotify,** साइन इन करने के बाद: आपकी लाइब्रेरी, आपके Spotify Connect
  डिवाइस, और Spotify का प्लेयर, जो `sdk.scdn.co` से लोड होता है, जब MixMusic
  में Spotify चलता है।
- **Better Lyrics:** चल रहे गाने का शीर्षक, पहला कलाकार, एल्बम और अवधि, उसके
  lyrics खोजने के लिए, जब उसके पास एल्बम और अवधि हो।
- **Unison:** चल रहे गाने की video ID, या उसके बिना, उसका शीर्षक, कलाकार, एल्बम
  और अवधि, उसके lyrics खोजने के लिए। सिर्फ़ तब जब आप lyrics के पास translate
  या अरबी का romanize चालू करें, चल रहे गाने के lyrics, उसकी video ID के साथ,
  जिन्हें Unison, Google से अनुवाद या romanize कराता है।
- **LRCLIB:** चल रहे गाने का शीर्षक, कलाकार, एल्बम और अवधि, उसके lyrics खोजने
  के लिए।
- **SponsorBlock,** जब आप Skip the parts SponsorBlock marks (SponsorBlock के
  चिह्नित हिस्से छोड़ें) चालू करें: हर video ID के hash के पहले 4 अक्षर, ID कभी
  नहीं, उसके चिह्नित हिस्से खोजने के लिए।
- **GitHub:** शुरू होने पर और हर कुछ घंटों में नए संस्करण की जाँच, जिसे आप
  Settings › About (सेटिंग › परिचय) में बंद कर सकते हैं; जब आप मार्केटप्लेस
  खोलते हैं तब समुदाय की registry; yt-dlp के डाउनलोड; AutoEq की हेडफ़ोन सूची और
  आपकी चुनी प्रोफ़ाइल; और, जब आप पहली बार जापानी lyrics के पास romaji चालू
  करते हैं, तब kanji के उच्चारण की डिक्शनरी (11.7 MB)। Romaji, और अरबी को छोड़
  दूसरी लिपियों के लैटिन अक्षर, आपके कंप्यूटर पर बनते हैं: lyrics भेजे नहीं
  जाते।
- **Last.fm,** जोड़ने के बाद: आप क्या सुनते हैं।
- **ListenBrainz,** या उसकी API वाला आपका अपना सर्वर, जोड़ने के बाद: आप क्या
  सुनते हैं।
- **Libre.fm,** या कोई दूसरा GNU FM सर्वर, जोड़ने के बाद: आप क्या सुनते हैं।
- **Discord,** जब तक आप इसे बंद न करें: जो चल रहा है, आपके कंप्यूटर के Discord
  ऐप को भेजा जाता है, जो उसे आपकी प्रोफ़ाइल पर दिखाता है।
- **जिन डिवाइस पर आप कास्ट करते हैं,** आपके नेटवर्क पर: चल रहे गाने का शीर्षक,
  कलाकार और कवर। वे ऑडियो MixMusic से लेते हैं।
- **MixMusic वाले दूसरे कंप्यूटर,** रिमोट कंट्रोल और दोस्तों के साथ सुनने के
  लिए, एन्क्रिप्ट होकर, आपके नेटवर्क पर या, जब आप उसके बाहर के दोस्तों को
  जुड़ने दें, इंटरनेट पर। तब आपका राउटर एक पोर्ट फ़ॉरवर्ड करने को कहा जाता है,
  और आपके आमंत्रण लिंक में आपका इंटरनेट पता होता है।
- **AI असिस्टेंट** आपके कंप्यूटर पर, जब तक आप उन्हें इजाज़त दें: जो चल रहा है,
  क्यू, उनकी सर्च के नतीजे और आपकी playlist के नाम, जिन्हें असिस्टेंट अपने
  इस्तेमाल किए AI मॉडल को आगे भेज सकता है।
- **YouTube और Spotify,** यह जानने के लिए कि नेटवर्क जवाब देता है या नहीं:
  HTTPS पर एक खाली जवाब का अनुरोध।

प्लगइन जो चाहें भेज सकते हैं: प्लगइन की अनुमतियाँ आपको दिखाई जाती हैं, लागू
नहीं की जातीं, इसीलिए MixMusic प्लगइन चालू करने से पहले आपसे उस पर भरोसा करने को
कहता है। [आपका डेटा और गोपनीयता](docs/data-and-privacy.md#what-each-feature-sends-where)
में एक तालिका है कि कौन-सी सुविधा क्या कहाँ भेजती है।

यह आपके कंप्यूटर पर क्या रखता है:

- आपकी सेटिंग और लाइब्रेरी (पसंद और नापसंद किए गाने, mix, हर वह play जो आपके
  आँकड़े गिनते हैं, फ़ॉलो किए कलाकार और शो, पिन, फ़ोल्डर और टैग, हिस्ट्री, हाल की
  सर्च, और आपकी संगीत फ़ाइलों का इंडेक्स), उस फ़ोल्डर में जिसका नाम Settings ›
  About बताता है।
- आपके सुने गाने, कैश में, उतने जितना आप Settings › Playback (सेटिंग ›
  प्लेबैक) में allow करें, और YouTube ने उन्हें कितना तेज़ नापा। आपके डाउनलोड
  किए गाने, डाउनलोड फ़ोल्डर में।
- आपके देखे कवर, 256 MB तक।
- आपके चालू किए lyrics के अनुवाद, गाने और भाषा के हिसाब से, और वह डिक्शनरी जिससे
  romaji, kanji पढ़ता है, एक बार चालू करने पर।
- आपके साइन-इन, YouTube Music का सेशन भी, सिस्टम की keyring में। Windows पर,
  जब Credential Manager किसी को नहीं रख पाता, तो वह आपके Windows अकाउंट के लिए
  एन्क्रिप्ट की गई फ़ाइल में जाता है; बिना keyring वाले Linux पर, ऐसी फ़ाइलों में
  जिन्हें सिर्फ़ आपका उपयोगकर्ता पढ़ सकता है।
- लॉग, आपके साइन-इन के बिना।

लाइब्रेरी का हिस्ट्री पेज आपके सुने गाने मिटाता है, Settings › Playback कैश और
कवर मिटाता है, और किसी सेवा से साइन आउट करने पर उसका साइन-इन मिट जाता है।
[मिटाएँ, हटाएँ या रीसेट करें](docs/data-and-privacy.md#clear-move-or-reset)
बताता है कि अपना डेटा कैसे ले जाएँ, `MIXMUSIC_DATA_DIR` से पोर्टेबल कॉपी कैसे
रखें, या नए सिरे से कैसे शुरू करें।

Windows पर, विज़ुअलाइज़र इस कंप्यूटर पर चल रहे Spotify का ऑडियो सुनता है, चाहे
MixMusic में हो या Spotify ऐप में, Windows के process loopback से, जो सिर्फ़ उसी
ऐप को सुनता है। macOS 14.2 और बाद के संस्करणों पर, यह MixMusic में चल रहे Spotify
को process tap से सुनता है, आपके allow करने के बाद, जैसा macOS पूछता है। यह ऑडियो
का विश्लेषण मेमोरी में करता है और उसे कभी रिकॉर्ड या भेजता नहीं।

## श्रेय

MixMusic इन प्रोजेक्ट पर बना है:

- **[MyGo](https://github.com/egoist/mygo)**, वह डेस्कटॉप फ़्रेमवर्क जिससे MixMusic
  बना है। MixMusic, MyGo का नेटिव UI इस्तेमाल करता है: MyGo हर विंडो खुद बनाता
  है, Go में, बिना webview के, पेज और प्लेयर बार से लेकर मिनी प्लेयर तक। इसकी
  कमांड लाइन ऐप को इंस्टॉलर और साइन किए हुए अपडेट के साथ पैकेज करती है, जिनसे
  वह खुद को अपडेट करता है।
- **[Go](https://go.dev)**, ऐप की भाषा, [purego](https://github.com/ebitengine/purego)
  के साथ, जो cgo के बिना सिस्टम की लाइब्रेरी को कॉल करता है, और
  [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) और
  [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)।
- **[Sobek](https://github.com/grafana/sobek)**, वह JavaScript इंजन जिसमें हर
  प्लगइन चलता है।
- **[Nix](https://nixos.org)**, जो रिलीज़ बनाने वाले टूल को पिन करता है, और
  MixMusic को Linux के लिए सोर्स से बनाता है।
- **[GoReleaser](https://goreleaser.com)**, जो रिलीज़ प्रकाशित करता है, और उसका
  [nFPM](https://nfpm.goreleaser.com), जो RPM पैकेज बनाता है।
- **[Bun](https://bun.sh)**, जो बिल्ड चलाता है, और
  **[TypeScript](https://www.typescriptlang.org)**, जो प्लगइन को कंपाइल करता है
  और प्लगइन SDK के types जाँचता है।
- **[Zola](https://www.getzola.org)**, जो वेबसाइट बनाता है।
- **[NSIS](https://nsis.sourceforge.io)**, जो Windows के इंस्टॉलर बनाता है, और
  [appimagetool](https://github.com/AppImage/appimagetool), AppImage बनाने वाला।
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, जो उन गानों का ऑडियो ढूँढता है
  जिन्हें MixMusic खुद नहीं ढूँढ पाता।
- **[SponsorBlock](https://sponsor.ajay.app)**, YouTube वीडियो के हिस्सों का खुला,
  crowdsourced डेटाबेस, Ajay Ramachandran और उसके योगदानकर्ताओं का, जिसके
  हिस्से MixMusic तब छोड़ता है जब आप कहें। Uses SponsorBlock data licensed under
  [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/)
  from https://sponsor.ajay.app/.
- **[Better Lyrics](https://betterlyrics.org)**, जिसकी सेवा शब्द-दर-शब्द समय वाले
  lyrics रखती है।
- **[Unison](https://unison.betterlyrics.org)**, lyrics का खुला, crowdsourced
  डेटाबेस, Unison Community License के तहत, और **[LRCLIB](https://lrclib.net)**, सिंक होने वाले
  lyrics का खुला डेटाबेस।
- **[kagome](https://github.com/ikawaha/kagome)**, जापानी morphological analyser
  जो romaji के लिए lyrics के kanji पढ़ता है, [kagome-dict](https://github.com/ikawaha/kagome-dict)
  की IPA डिक्शनरी के साथ, जो mecab-ipadic से बनी है, और जिसे MixMusic romaji
  पहली बार चालू होने पर डाउनलोड करता है।
- **[libopus](https://opus-codec.org)**, जिसके डिकोडर का Opus डिकोडर एक पोर्ट है,
  और **[SharpJaad](https://github.com/jimm98y/SharpJaad)**, [JAAD](https://sourceforge.net/projects/jaadec/)
  का C# पोर्ट, जिससे AAC डिकोडर निकला है।
- **[Chromium](https://www.chromium.org)** का Web Audio, जिसके biquad,
  DynamicsCompressor और AnalyserNode के पोर्ट इक्वलाइज़र के फ़िल्टर, वॉल्यूम
  लेवलिंग और विज़ुअलाइज़र हैं।
- **[AutoEq](https://github.com/jaakkopasanen/AutoEq)**, Jaakko Pasanen और उसके
  योगदानकर्ताओं का, जिसकी प्रोफ़ाइल हेडफ़ोन की आवाज़ सुधारती हैं,
  [Equalizer APO](https://sourceforge.net/projects/equalizerapo/) के प्रारूप में।
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)** और
  **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis)**, जो आपके कंप्यूटर की
  MP3 और Vorbis फ़ाइलें डिकोड करते हैं, और **[Xiph.Org](https://xiph.org)**,
  जिसके FLAC और Ogg को `flac` और `ogg` पढ़ते हैं।
- **[FFmpeg](https://ffmpeg.org)**, जिससे डिकोडरों के टेस्ट तुलना करते हैं।
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** और
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, विंडो के फ़ॉन्ट।
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  जिसके HCT कलर साइंस का पोर्ट Material थीम के रंग हैं।
- **[FCast](https://docs.fcast.org)**, वह खुला प्रोटोकॉल जिससे MixMusic FCast
  रिसीवर पर चलाता है। MixMusic उसका एक स्वतंत्र कार्यान्वयन है, और FUTO से जुड़ा
  नहीं है; FCast FUTO का पंजीकृत ट्रेडमार्क है।
- **[Kevin MacLeod](https://incompetech.com)**, जिनका संगीत,
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) के तहत, वेबसाइट पर
  बजता है।

[Credits](docs/credits.md) हर उस प्रोजेक्ट, शोधपत्र और मानक का नाम बताता है जिन
पर MixMusic बना है।

## योगदान करें

सुधार, नई सुविधाएँ, थीम, प्लगइन और बग रिपोर्ट, सभी का स्वागत है।

- **समस्या बताएँ** Settings › About › report a problem (सेटिंग › परिचय ›
  समस्या बताएँ) से। यह आपके संस्करण, सिस्टम, थीम और प्लगइन भरे हुए एक नया GitHub
  issue खोलता है। या खुद [एक issue खोलें](https://github.com/fzn0x/mixmusic/issues/new)।
- **बदलाव भेजें** pull request के रूप में। [CONTRIBUTING.md](CONTRIBUTING.md)
  बताता है कि कौन-सी जाँचें चलानी हैं, commit की शैली क्या है, और ऐसे बदलाव के
  लिए changelog की कौन-सी प्रविष्टि चाहिए जिसे लोग नोटिस करेंगे।
- **थीम या प्लगइन साझा करें:** देखें [समुदाय](#समुदाय)।
- **सुरक्षा कमज़ोरी की रिपोर्ट** निजी तौर पर करें, जैसा [SECURITY.md](SECURITY.md)
  बताता है।

हिस्सा लेने वाला हर व्यक्ति [आचार संहिता](CODE_OF_CONDUCT.md) का पालन करता है।
[CONTRIBUTORS.md](CONTRIBUTORS.md) उन लोगों का धन्यवाद करता है जिनके कोड, विचारों
और रिपोर्ट ने MixMusic को आकार दिया।

## लाइसेंस

MixMusic दोहरे लाइसेंस के तहत है: इसे [MIT लाइसेंस](LICENSE-MIT) या
[GNU जनरल पब्लिक लाइसेंस, संस्करण 3](LICENSE-GPL) के तहत इस्तेमाल करें, जो भी
आपको पसंद हो। योगदान दोनों के तहत स्वीकार किए जाते हैं। `fonts` में मौजूद
फ़ॉन्ट SIL Open Font License के तहत हैं: [Selawik के](fonts/OFL.txt),
[Quicksand के](fonts/quicksand-OFL.txt),
[Roboto Flex के](fonts/robotoflex-OFL.txt) और
[Archivo के](fonts/archivo-OFL.txt)। Opus डिकोडर libopus का पोर्ट है,
[उसके BSD लाइसेंस](opus/LICENSE.libopus) के तहत, AAC डिकोडर SharpJaad से निकला
है, [उसके MIT लाइसेंस](aac/LICENSE.sharpjaad) के तहत, और HCT कलर साइंस Material
Color Utilities का पोर्ट है, [उसके Apache लाइसेंस](internal/hct/LICENSE.material-color-utilities)
के तहत। बाकी हिस्से उन चीज़ों के लाइसेंस रखते हैं जिनसे वे निकले हैं, जैसे
Chromium के Web Audio के फ़िल्टर, go-mp3, jfreymuth/vorbis, kagome और Unihan
डेटा: [License](docs/license.md) हर एक को गिनाता है।

MixMusic का Spotify या Google से कोई संबंध नहीं है, और न ही इन्होंने इसे
समर्थन या प्रायोजित किया है। यह वही चलाता है जो ये सेवाएँ अपने उपयोगकर्ताओं को
देती हैं, सेवाओं के अपने इंटरफ़ेस के ज़रिए; इसे वैसे ही इस्तेमाल करें जैसा उनकी
शर्तें अनुमति देती हैं। Spotify और YouTube Music अपने-अपने मालिकों के ट्रेडमार्क
हैं।
