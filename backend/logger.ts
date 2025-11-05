import pino from 'pino';
export const logger = pino({
    transport: {
        target: 'pino-pretty'
    },

    // formatters: {
    //     level: (label) => {
    //         return { level: label.toUpperCase() };
    //     },
    // },
    // timestamp: pino.stdTimeFunctions.isoTime,
},
    // pino.destination(`${ROOT_DIR}/app.log`)
)