import {Hono} from "hono";

const app = new Hono();

app.get('/', (c) => {
    return c.text('Hello Deployment');
})

export default app;