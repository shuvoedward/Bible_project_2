import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
    vus: 1,
    duration: "10s",
    thresholds: {
        http_req_duration: ["p(95)<500"],
        http_req_failed: ["rate<0.01"],
    },
};

export function setup() {
    const res = http.get("http://localhost:4000/v1/healthcheck");
    if (res.status !== 200) {
        throw new Error("Api not running");
    }
}

export default function () {
    const res = http.get("http://localhost:4000/v1/bible/Genesis/1");

    check(res, {
        "status is 200": (r) => r.status === 200,
        "body is not empty": (r) => r.body.length > 0,
    });

    sleep(1);
}
