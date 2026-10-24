# Firebase Auth Emulator（ローカル開発・テスト用）
# Auth の Emulator は Java を必要としないため、Node のイメージだけで動く。
FROM node:22-slim

RUN npm install -g firebase-tools@15.33.0

WORKDIR /firebase
COPY firebase.json .

EXPOSE 9099

# "demo-" で始まるプロジェクト ID は、実在の Firebase プロジェクトなしで Emulator だけで動く
CMD ["firebase", "emulators:start", "--only", "auth", "--project", "demo-uchinokusa"]
