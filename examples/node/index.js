import express from 'express';

const app = express();

app.get('/', (req, res) => {
    res.send('Hello Deployment');
});

app.listen(3000);